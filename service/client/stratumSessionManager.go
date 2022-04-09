package client

import (
	"amenity.com/ragent-client/kit"
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// r-client 协议
const (
	RegisterMiner       = 0x01
	AuthenticationMiner = 0x02
	SubmitShare         = 0x03
	MachineOffline      = 0x06
	PENETRATE           = 0x07

	S2cDispatchJob             = 0x04
	S2cMiningSetDiff           = 0x05
	S2cDispatchWorkerJob       = 0x08
	S2cDispatchCommonWorkerJob = 0x09

	S2cRegisterMiner       = 0x81
	S2cAuthenticationMiner = 0x82
	S2cSubmitShare         = 0x83
	S2cMachineOffline      = 0x86
	S2cPenetrate           = 0x87
)

// RaMagic 协议规定的MAGIC NUMBER
const RaMagic = 0x2B

// CommonSendByteLength 协议规定的发送客户端长度 新增serverID 4 个长度

const CommonSendByteLength = 12 + 4

type StratumSession struct {
	isRunning    bool
	isShowNotify bool
	minerConn    net.Conn
	minerReader  *bufio.Reader

	startTime       int64
	extraNonce1     string
	extraNonce2size int64

	ssm        *StratumSessionManager
	agent      string
	password   string
	workerName string

	lock      sync.Mutex
	wg        sync.WaitGroup
	requestID interface{}
	reqNo     string
}

const (
	rSubmitShare = "mining.submit"
	rSubscribe   = "mining.subscribe"
	rAuthorize   = "mining.authorize"
)

func (ss *StratumSession) runListenMiner() {
	defer ss.MinerClear()
	for ss.IsRunning() {
		buf, err := ss.minerReader.ReadBytes('\n')
		zap.L().Info("MinerMessageRead", zap.String("ReqNo", ss.reqNo), zap.ByteString("Data", buf), zap.Int("MsgLength", len(buf)))
		if err != nil {
			zap.L().Error("Miner Read Err ", zap.Error(err), zap.String("ReqNo", ss.reqNo))
			// 处理矿机离线
			ss.handleMinerOffline()
			break
		}

		// 客户端会莫名其妙发一些空消息, 这里直接选择忽略长度过低的消息, 否则就会走踢矿机下线的逻辑了
		if len(buf) < 3 {
			zap.L().Error("Miner Send Error Msg", zap.String("ReqNo", ss.reqNo))
			continue
		}

		ss.processMinerMsg(buf)
	}
}

// MinerClose 断开miner连接
func (ss *StratumSession) MinerClose() {
	if !ss.IsRunning() {
		return
	}

	err := ss.minerConn.Close()
	if err != nil {
		zap.L().Error("Miner Closed Error ", zap.Error(err), zap.String("ReqNo", ss.reqNo))
	}
	ss.lock.Lock()
	ss.isRunning = false
	ss.lock.Unlock()
	zap.L().Info("Miner offline", zap.Uint64("ServerId", ss.ssm.ServerId), zap.String("ReqNo", ss.reqNo))
}

// MinerClear 清理map连接
func (ss *StratumSession) MinerClear() {

	if ss.IsRunning() {
		ss.MinerClose()
	}

	ss.ssm.StratumSessionMapLock.Lock()
	delete(ss.ssm.StratumSessionMap, fmt.Sprintf("%s", ss.reqNo))
	ss.ssm.StratumSessionMapLock.Unlock()
}

func (ss *StratumSession) processMinerMsg(buf []byte) {
	zap.L().Info("Miner Msg:", zap.String("ReqNo:", ss.reqNo), zap.ByteString("Msg Byte", buf))
	req := new(JSONRPCRequest)
	err := json.Unmarshal(buf, req)
	if err != nil {
		zap.L().Error("Miner JSON DECODE ERROR Msg : ", zap.String("ReqNo:", ss.reqNo), zap.ByteString("Msg Byte", buf))
	}

	switch req.Method {
	case rSubmitShare:
		ss.handleSubmitShare(req)
	case rAuthorize:
		ss.handleAuthorize(req)
	case rSubscribe:
		ss.handleSubscribe(req)
	default:
		ss.handlePenetrate(req, buf)
	}
}

// 提交share
func (ss *StratumSession) handleSubmitShare(req *JSONRPCRequest) {
	zap.L().Info(
		"Miner handleSubmitShare : ",
		zap.String("ReqNo:", ss.reqNo),
		zap.Any("Msg Data", req),
		zap.Any("params", req.Params),
	)

	jobId := []byte(req.Params[1].(string))
	extraNonce2 := make([]byte, 20)
	paramsExtraNonce2 := []byte(req.Params[2].(string))

	// 定长20 byte 的只能一个个赋值
	for i := 0; i < len(extraNonce2); i++ {
		if i >= len(paramsExtraNonce2) {
			break
		}
		extraNonce2[i] = paramsExtraNonce2[i]
	}

	nTime := make([]byte, 4)
	nt, err := strconv.ParseUint(req.Params[3].(string), 16, 32)
	if err != nil {
		return
	}
	binary.LittleEndian.PutUint32(nTime, uint32(nt))

	nonce := make([]byte, 4)
	nn, err := strconv.ParseUint(req.Params[4].(string), 16, 32)
	if err != nil {
		return
	}
	binary.LittleEndian.PutUint32(nonce, uint32(nn))
	f := []byte{0x0}
	var bytesBuff bytes.Buffer
	if len(req.Params) == 5 {
		bytesBuff = ss.handleCommonData(SubmitShare, req.ID, nonce, extraNonce2, jobId, f, nTime)
	} else if len(req.Params) == 6 {
		nVersion := make([]byte, 4)
		nv, err := strconv.ParseUint(req.Params[5].(string), 16, 32)
		if err != nil {
			return
		}
		binary.LittleEndian.PutUint32(nVersion, uint32(nv))
		bytesBuff = ss.handleCommonData(SubmitShare, req.ID, nonce, extraNonce2, jobId, f, nTime, nVersion)
	}
	ss.send2Server(bytesBuff)
}

// 矿机认证
func (ss *StratumSession) handleAuthorize(req *JSONRPCRequest) {
	zap.L().Info("Miner handleAuthorize : ", zap.String("ReqNo:", ss.reqNo), zap.Any("Msg Data", req))
	workerName := []byte(req.Params[0].(string))
	password := []byte(req.Params[1].(string))
	f := []byte{0x0}
	bytesBuff := ss.handleCommonData(AuthenticationMiner, req.ID, workerName, f, password, f)
	ss.send2Server(bytesBuff)
}

// 矿机注册
func (ss *StratumSession) handleSubscribe(req *JSONRPCRequest) {
	zap.L().Info("Miner handleSubscribe : ", zap.String("ReqNo:", ss.reqNo), zap.Any("Msg Data", req))
	if req.Params == nil || len(req.Params) == 0 {
		return
	}
	ss.workerName = req.Params[0].(string)
	bytesBuff := ss.handleCommonData(RegisterMiner, req.ID, []byte(ss.workerName), []byte{0x0})
	ss.send2Server(bytesBuff)
}

// 透传
func (ss *StratumSession) handlePenetrate(req *JSONRPCRequest, buf []byte) {
	zap.L().Info("Miner handlePenetrate : ", zap.String("ReqNo:", ss.reqNo), zap.Any("Msg Data", req))
	bytesBuff := ss.handleCommonData(PENETRATE, req.ID, buf)
	ss.send2Server(bytesBuff)
}

// 处理矿机离线
func (ss StratumSession) handleMinerOffline() {

	req := new(JSONRPCRequest)
	zap.L().Info("Miner handleMinerOfflineStart:", zap.String("ReqNo:", ss.reqNo), zap.Any("Msg Data", req))
	bytesBuff := ss.handleCommonData(MachineOffline, req.ID, []byte{})
	// 写完之后直接传到对应Pool矿池
	ss.send2Server(bytesBuff)

	// 停2S 再关闭
	time.Sleep(time.Second * 2)
	if ss.IsRunning() {
		ss.MinerClose()
	}
	zap.L().Info("Miner handleMinerOfflineEnd:", zap.String("ReqNo:", ss.reqNo))
}

// 获取通用发送的数组
func (ss *StratumSession) handleCommonData(method byte, reqId interface{}, buf ...[]byte) (bytesBuff bytes.Buffer) {

	res := strings.Split(ss.reqNo, "-")
	buffs := bytes.Join(buf, []byte(""))

	messageLen := CommonSendByteLength + len(buffs)
	messageLenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(messageLenBytes, uint16(messageLen))

	serverId := make([]byte, 4)
	binary.LittleEndian.PutUint32(serverId, cast.ToUint32(res[0]))

	reqNo := make([]byte, 4)
	binary.LittleEndian.PutUint32(reqNo, cast.ToUint32(res[1]))
	reqIdByte := make([]byte, 4)
	binary.LittleEndian.PutUint32(reqIdByte, cast.ToUint32(reqId))

	bytesBuff.WriteByte(RaMagic)
	bytesBuff.WriteByte(method)
	bytesBuff.Write(messageLenBytes)
	bytesBuff.Write(serverId)
	bytesBuff.Write(reqNo)
	bytesBuff.Write(reqIdByte)

	bytesBuff.Write(buffs)
	zap.L().Debug(
		"handleCommonData",
		zap.Any("method", method),
		zap.Any("reqId", reqId),
		zap.Any("messageLen", messageLen),
		zap.Any("CommonSendByteLength", CommonSendByteLength),
		zap.Any("len(buffs)", len(buffs)),
		zap.Any("buffs", string(buffs)),
	)
	return
}

func (ss *StratumSession) send2Server(buff bytes.Buffer) {
	Send2ServerMsgChan <- Send2ServerMsg{
		msg: buff.Bytes(),
		sm:  *ss.ssm.ServerManager,
	}
}

func (ss *StratumSession) Write2Miner(data []byte) {
	st := time.Now()
	zap.L().Debug("Write2MinerStart", zap.ByteString("data", data), zap.String("ReqNo:", ss.reqNo), zap.Time("Send2MinerTime", st))
	ss.minerConn.Write(data)
	zap.L().Debug("Write2MinerEnd", zap.String("ReqNo:", ss.reqNo), zap.Int64("SpendTime", time.Now().Sub(st).Nanoseconds()))
}

type StratumSessionManager struct {
	// reqNo StratumSession Map
	StratumSessionMap     map[string]*StratumSession
	StratumSessionMapLock sync.RWMutex
	ServerId              uint64
	ServerManager         *Manager
}

var BuffReaderBufSize = 1024

// NewStratumSession 接收到矿机链接后主动链接
func (m *StratumSessionManager) NewStratumSession(minerConn net.Conn) (s *StratumSession) {

	reqNo := m.GenerateReqNo()
	s = &StratumSession{
		minerConn:   minerConn,
		minerReader: bufio.NewReaderSize(minerConn, BuffReaderBufSize),
		isRunning:   false,
		reqNo:       reqNo,
		startTime:   time.Now().Unix(),
	}
	m.StratumSessionMapLock.Lock()
	m.StratumSessionMap[reqNo] = s
	defer m.StratumSessionMapLock.Unlock()
	s.ssm = m
	return s
}

func (m *StratumSessionManager) GenerateReqNo() string {
	return fmt.Sprintf("%d-%d", m.ServerId, kit.GetCommonReqNo())
}

func NewStratumSessionManager(serverId uint64, serverManager *Manager) *StratumSessionManager {
	ssm := new(StratumSessionManager)
	ssm.StratumSessionMap = make(map[string]*StratumSession, 0)
	ssm.ServerId = serverId
	ssm.ServerManager = serverManager
	return ssm
}
