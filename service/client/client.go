package client

import (
	"amenity.com/ragent-client/kit/cache"
	"amenity.com/ragent-client/kit/cache/gredis"
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"time"

	"amenity.com/ragent-client/config"
	"amenity.com/ragent-client/log"
	"go.uber.org/zap"
)

var Send2ServerMsgChan chan Send2ServerMsg

type Send2ServerMsg struct {
	msg []byte
	sm  Manager
}

const Send2ServerMsgLen = 4028

func NewClientInstance(configPath, defaultConfigFlag, logDir string) *RAgentClient {

	if Rac != nil {
		return Rac
	}

	Rac = new(RAgentClient)
	if defaultConfigFlag == "true" {
		Rac.Config = config.GetDefaultConfig()
	} else {
		Rac.Config = config.LoadConfig(configPath)
	}
	Send2ServerMsgChan = make(chan Send2ServerMsg, Send2ServerMsgLen)
	//zapLog
	if len(logDir) > 0 {
		Rac.Config.Logger.Filename = logDir + Rac.Config.Logger.Filename
	}

	rand.Seed(time.Now().UnixNano())
	str := fmt.Sprintf("%d%d", Rac.Config.ServerId, rand.Intn(10000))
	Rac.Config.ServerId, _ = strconv.ParseUint(str, 10, 32)

	err := cache.InitRedis(Rac.Config.Redis.Address, Rac.Config.Redis.Port, Rac.Config.Redis.Password, gredis.RedisCache, cache.MaxIdle(Rac.Config.Redis.OpMaxActive), cache.IdleTimeout(Rac.Config.Redis.OpIdleTimeout), cache.MaxActive(Rac.Config.Redis.OpMaxActive))
	if err != nil {
		zap.L().Fatal("Agent Redis Not Found")
	}

	Rac.ZapLog = log.NewZapLog(Rac.Config.Logger.Filename,
		Rac.Config.Logger.MaxSize,
		Rac.Config.Logger.MaxBackups,
		Rac.Config.Logger.MaxAge,
		Rac.Config.Logger.Level,
		Rac.Config.Logger.Verbose,
		Rac.Config.Logger.Compress,
		[]zap.Field{zap.Uint64("serverId", Rac.Config.ServerId)})

	Rac.serverM = Rac.NewClientManager()
	Rac.Sm = NewStratumSessionManager(Rac.Config.ServerId, Rac.serverM)
	Rac.tcpListenAddr = fmt.Sprintf("%s:%d", Rac.Config.ListenIp, Rac.Config.ListenPort)

	Rac.RedisCache = gredis.NewCache()

	// 开启对应发送信息到server的服务
	go Rac.sendMsg2Server()
	go Rac.ReadFromServer()
	return Rac
}

func (r *RAgentClient) NewClientManager() (sm *Manager) {
	// 测试链接 server 服务地址，如果不同则报错
	addr := ""
	if r.serverM != nil {
		addr = r.serverM.serverAddr
	}
	sm, err := r.ChooseServer(addr)

	if err != nil {
		// 如果选择Server 报错，就将矿机设置为无链接
		zap.L().Error("Client ChoosePool Error : ", zap.Error(err))
	}
	return sm
}

// TryFirstServe 判断配置文件的第一个server是否可用, 如果可用则切换到第一个 server
func (r *RAgentClient) TryFirstServe() (sm *Manager, err error) {
	firstServerAddr := fmt.Sprintf("%s:%d", r.Config.Servers[0].Ip, r.Config.Servers[0].Port)

	conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, e := tls.Dial("tcp", firstServerAddr, conf)
	if e != nil || conn == nil {
		zap.L().Error("TryFirstServe Connect Pool Error ", zap.Error(e))
		return nil, e
	}

	sm = new(Manager)
	sm.serverConn = conn
	sm.serverAddr = firstServerAddr

	sm.serverStartTime = time.Now().Unix()
	sm.serverPool = &Server{
		port: r.Config.Servers[0].Port,
		ip:   r.Config.Servers[0].Ip,
	}
	// 标记是否已链接
	sm.isConn = true
	sm.serverReader = bufio.NewReaderSize(sm.serverConn, BuffReaderBufSize)

	r.closeAllMiner()

	return sm, nil
}

// ChooseServer 从配置项目中选择一个并尝试链接
func (r *RAgentClient) ChooseServer(lastPoolAddr string) (sm *Manager, err error) {
	sm = new(Manager)
	configServerCount := len(r.Config.Servers)
	if configServerCount <= 0 {
		return nil, errors.New("Config:Server pool config can not be null")
	}
	TotalTryTimes := 3 * configServerCount
	current := 0
	// 重试 len(server) * 3 次
	for TotalTryTimes-current > 0 {
		// 从server列表中随机获取一个

		i := rand.Int31n(int32(configServerCount))
		current++

		addr := fmt.Sprintf("%s:%d", r.Config.Servers[i].Ip, r.Config.Servers[i].Port)
		// 如果和当前使用的server重复, 则继续尝试随机获取
		if lastPoolAddr == addr {
			continue
		}

		conf := &tls.Config{
			InsecureSkipVerify: true,
		}

		conn, e := tls.Dial("tcp", addr, conf)

		if e != nil {
			zap.L().Error("Connect "+addr+" Pool Error ", zap.Error(e))
			continue
		}

		if conn != nil {
			sm.serverConn = conn
			sm.serverAddr = addr

			sm.serverStartTime = time.Now().Unix()
			sm.serverPool = &Server{
				port: r.Config.Servers[i].Port,
				ip:   r.Config.Servers[i].Ip,
			}
			// 标记是否已链接
			sm.isConn = true
			sm.serverReader = bufio.NewReaderSize(sm.serverConn, BuffReaderBufSize)
			r.closeAllMiner()
			return sm, nil
		}
	}
	// 重试次数达到break的时候,还是没有切换server,则继续使用之前的server
	return sm, errors.New("Config:Server pool config no one can be used")
}

func (r *RAgentClient) closeAllMiner() {

	if r.Sm == nil {
		return
	}

	// 防止踢矿机过程中,矿机重连
	r.serverM.isConn = false
	// 关闭所有矿机连接
	for _, v := range r.Sm.StratumSessionMap {
		// 断开本地矿机的连接
		r.MinerOffline(v)
	}
	// 断开服务端连接(服务端会自动踢当前client所有矿机)
	if r.Sm.ServerManager.serverConn != nil {
		_ = r.Sm.ServerManager.serverConn.Close()
	}

}

func (r *RAgentClient) sendMsg2Server() {
	defer func() {
		zap.L().Error("sendMsg2Server closed")
	}()
	for {

		if !r.serverM.isConn {
			time.Sleep(time.Second)
			continue
		}

		d, ok := <-Send2ServerMsgChan
		zap.L().Info("Msg2Server", zap.ByteString("Data", d.msg), zap.String("ServerUrl", d.sm.serverAddr))
		if !ok {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		//go func() {
		//	if err := recover(); err != nil {
		//		zap.L().Error("GOROUTINE RECOVERED: sendMsg2Server panic", zap.Any("Error Reason : ", err))
		//	}
		d.sm.serverConn.Write(d.msg)
		//}()
	}
}

func (r *RAgentClient) ReadFromServer() {
	// 然后一直不停的读取conn的信息
	for {

		if !r.serverM.isConn {
			time.Sleep(time.Second)
			continue
		}

		msg, err := r.ReadMessage(r.serverM.serverReader)
		if err != nil {
			// 处理EOF
			if err.Error() == "EOF" {
				zap.L().Error("ReadFromServer EOF", zap.Error(err))
				r.serverM.isConn = false
				r.serverM.serverAddr = ""
				for _, v := range r.Sm.StratumSessionMap {
					r.MinerOffline(v)
				}
				continue
			}
			zap.L().Error("ReadFromServerMsgErr", zap.Error(err))
			time.Sleep(time.Millisecond * 10)
			continue
		}
		r.DoByMsg(msg)
	}
}

func (r *RAgentClient) DoByMsg(msg *ServerRequest) {
	action := msg.MessageType

	var ss *StratumSession
	r.Sm.StratumSessionMapLock.RLock()
	ss = r.Sm.StratumSessionMap[fmt.Sprintf("%d-%d", msg.ServerId, msg.ReqNo)]
	r.Sm.StratumSessionMapLock.RUnlock()
	if ss == nil {
		zap.L().Error("StratumSession Is Nil ", zap.Uint32("ServerId", msg.ServerId), zap.Uint32("ReqNo", msg.ReqNo))
		return
	}
	lastMagic = msg.MessageType
	_ = msg.ServerId
	lastReqNo = msg.ReqNo
	lastRequestId = msg.RequestId

	switch action {
	case S2cSubmitShare:
		// 提交share响应
		r.submitShare(ss, msg)
	case S2cDispatchWorkerJob:
		// 提交share响应
		r.miningNotify(ss, msg)
	case S2cDispatchCommonWorkerJob:
		// 提交share响应
		r.miningNotifyCommon(ss, msg)
	case S2cDispatchJob:
		// 提交share响应
		r.miningNotifyAll(msg)
	case S2cMiningSetDiff:
		// 提交share响应
		r.setDiff(ss, msg)
	case S2cAuthenticationMiner:
		// 矿机授权响应
		r.authMiner(ss, msg)
	case S2cRegisterMiner:
		// 矿机注册响应
		r.registerMiner(ss, msg)
	case S2cPenetrate:
		// 透传
		r.penetrate(ss, msg)
	case S2cMachineOffline:
		// 服务端通知矿机下线
		r.MinerOffline(ss)
	}
}

func (r *RAgentClient) Run() (err error) {
	if r.tcpListener == nil {
		zap.L().Info("listen tcp", zap.String("listenAddr", r.tcpListenAddr))
		r.tcpListener, err = net.Listen("tcp", r.tcpListenAddr)
		if err != nil {
			zap.L().Fatal("Server Listen Failed", zap.Error(err))
			return
		}
	}

	for {
		// 如果没链接server 不接收client
		if !r.serverM.isConn {
			time.Sleep(time.Second)
			continue
		}

		conn, err := r.tcpListener.Accept()
		zap.L().Warn("New Miner Coming", zap.String("Miner IP", conn.RemoteAddr().String()))
		if err != nil {
			zap.L().Warn("New Miner Coming Error", zap.Error(err))
			_ = conn.Close()
			continue
		}

		if conn != nil {
			s := r.Sm.NewStratumSession(conn)
			s.Start()
		}
		/* prevent from lots of coming connection on restart client */
		time.Sleep(1 * time.Millisecond)
	}
}

func (r *RAgentClient) ReadMessage(src *bufio.Reader) (sr *ServerRequest, err error) {

	magic, err := src.ReadByte()
	if err != nil {
		zap.L().Error("Read Magic Error", zap.Any("MessageType", lastMagic), zap.Uint32("LastReqNo", lastReqNo),
			zap.Uint32("LastRequestId", lastRequestId), zap.String("Pool Addr ", r.serverM.serverAddr))
		return
	}

	if magic != RaMagic {
		zap.L().Error("LastMagicMessageType", zap.Any("MessageType", lastMagic), zap.Uint32("LastReqNo", lastReqNo),
			zap.Uint32("LastRequestId", lastRequestId), zap.String("Server Addr ", r.serverM.serverAddr))

		err = errors.New("incorrect magic")
		return
	}

	messageType, err := src.ReadByte()
	if err != nil {
		zap.L().Error("Read MessageType Error", zap.Any("MessageType", lastMagic), zap.Uint32("LastReqNo", lastReqNo),
			zap.Uint32("LastRequestId", lastRequestId), zap.String("Server Addr ", r.serverM.serverAddr))
		return
	}

	var messageLen uint16
	err = binary.Read(src, binary.LittleEndian, &messageLen)

	if err != nil {
		return
	}
	var serverId uint32
	binary.Read(src, binary.LittleEndian, &serverId)

	var reqNo uint32
	binary.Read(src, binary.LittleEndian, &reqNo)

	var requestId uint32
	binary.Read(src, binary.LittleEndian, &requestId)

	lef := messageLen - CommonSendByteLength
	// 不管怎么样都要读完
	byt := make([]byte, lef)
	io.ReadFull(r.serverM.serverReader, byt)

	sr = &ServerRequest{
		Magic:       magic,
		MessageType: messageType,
		Length:      messageLen,
		ServerId:    serverId,
		ReqNo:       reqNo,
		RequestId:   requestId,
		Data:        byt,
	}

	return sr, nil
}
