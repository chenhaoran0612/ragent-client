package client

import (
	"amenity.com/ragent-client/config"
	"amenity.com/ragent-client/kit/cache/gredis"
	"amenity.com/ragent-client/log"
	"amenity.com/ragent-client/service/stat"
	"amenity.com/ragent-client/service/upgrade"
	"bufio"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"net"
	"time"
)

var lastMagic byte
var lastReqNo uint32
var lastRequestId uint32

// Rac Global Server
var Rac *RAgentClient

const DefaultConnectTimeOut = 3000

type Manager struct {
	serverAddr      string
	serverReader    *bufio.Reader
	serverStartTime int64
	serverPool      *Server
	serverConn      net.Conn
	isConn          bool
}

type RAgentClient struct {
	Config        *config.Config
	StatSvc       *stat.StatService
	Sm            *StratumSessionManager
	serverM       *Manager
	UpgradeSvc    *upgrade.UpgradeService
	ZapLog        *log.ZapLog
	tcpListener   net.Listener
	tcpListenAddr string
	//BDB           *buntdb.DB
	RedisCache gredis.Cache
}

func (r *RAgentClient) penetrate(ss *StratumSession, msg *ServerRequest) {
	zap.L().Debug(
		"PENETRATE",
		zap.Uint32("ReqNo", msg.ReqNo),
		zap.Uint32("ServerId", msg.ServerId),
		zap.Any("MSG", msg),
		zap.Int32("RequestId", int32(msg.RequestId)),
		zap.String("penetrateByt", string(msg.Data)),
	)
	if ss != nil && ss.minerConn != nil {
		ss.Write2Miner(msg.Data)
	}
}

// 响应矿机注册
func (r *RAgentClient) registerMiner(ss *StratumSession, msg *ServerRequest) {
	zap.L().Debug(
		"RegisterMiner",
		zap.Uint32("ReqNo", msg.ReqNo),
		zap.Uint32("ServerId", msg.ServerId),
		zap.Any("MSG", msg),
		zap.Int32("RequestId", int32(msg.RequestId)),
	)
	var extraNonce2Size uint16
	extraNonce2Size = binary.LittleEndian.Uint16(msg.Data[0:2])
	diff := string(msg.Data[2 : len(msg.Data)-1])
	result1 := [][]string{
		{"mining.set_difficulty", diff}, {"mining.notify", diff},
	}
	res := make([]interface{}, 3)
	res[0] = result1
	res[1] = diff
	res[2] = extraNonce2Size
	rep := JSONRPCResponse{ID: msg.RequestId, Result: res, Error: nil}
	buf, err := rep.ToJSONBytes()
	if err != nil {
		return
	}
	zap.L().Info("JRPC Subscribe", zap.String("BUF", string(buf)))
	if ss != nil && ss.minerConn != nil {
		ss.Write2Miner(append(buf, byte('\n')))
		//ss.minerConn.Write()
	}
}

// 矿机授权响应
func (r *RAgentClient) authMiner(ss *StratumSession, msg *ServerRequest) {
	zap.L().Debug(
		"AuthMiner",
		zap.Uint32("ReqNo", msg.ReqNo),
		zap.Uint32("ServerId", msg.ServerId),
		zap.Any("MSG", msg),
		zap.Int32("RequestId", int32(msg.RequestId)),
	)
	var (
		res  uint16
		flag bool = true
	)
	res = binary.LittleEndian.Uint16(msg.Data[0:2])
	if res == uint16(0) {
		flag = false
	}
	rep := JSONRPCResponse{ID: msg.RequestId, Result: flag, Error: nil}
	buf, err := rep.ToJSONBytes()
	if err != nil {
		return
	}
	if ss != nil && ss.minerConn != nil {
		ss.Write2Miner(append(buf, byte('\n')))
		//ss.minerConn.Write(append(buf, byte('\n')))
	}
}

// 响应提交share
func (r *RAgentClient) submitShare(ss *StratumSession, msg *ServerRequest) {
	zap.L().Debug(
		"SubmitShare",
		zap.Uint32("ReqNo", msg.ReqNo),
		zap.Uint32("ServerId", msg.ServerId),
		zap.Any("MSG", msg),
		zap.Int32("RequestId", int32(msg.RequestId)),
	)
	var (
		flag   uint16
		reason string
		result bool
	)
	flag = binary.LittleEndian.Uint16(msg.Data[0:2])
	reason = string(msg.Data[2 : len(msg.Data)-1])
	if flag == uint16(1) {
		result = true
	}
	rep := JSONRPCResponse{ID: msg.RequestId, Result: result, Error: reason}
	buf, err := rep.ToJSONBytes()
	if err != nil {
		return
	}
	if ss != nil && ss.minerConn != nil {
		ss.Write2Miner(append(buf, byte('\n')))
		//ss.minerConn.Write(append(buf, byte('\n')))
	}
}

func (r *RAgentClient) miningNotifyAll(msg *ServerRequest) {
	zap.L().Debug(
		"MiningNotifyAll",
		zap.Uint32("ServerId", msg.ServerId),
		zap.Any("Data", msg.Data),
		zap.Int32("RequestId", int32(msg.RequestId)),
	)
	// 所有的都发一遍
	for _, session := range r.Sm.StratumSessionMap {
		if session.isShowNotify && session.isRunning {
			session.Write2Miner(append(msg.Data, byte('\n')))
			//session.minerConn.Write(append(msg.Data, byte('\n')))
		}
	}
}

func (r *RAgentClient) miningNotify(ss *StratumSession, msg *ServerRequest) {
	zap.L().Debug(
		"MiningNotify",
		zap.Uint32("ReqNo", msg.ReqNo),
		zap.Uint32("ServerId", msg.ServerId),
		zap.ByteString("MSG", msg.Data),
		zap.Int32("RequestId", int32(msg.RequestId)),
	)
	sendNotify := new(JSONRPCRequest)
	err := json.Unmarshal(msg.Data, &sendNotify)
	if err != nil {
		zap.L().Error(
			"SendNotify Unmarshal Error",
			zap.Uint32("ReqNo", msg.ReqNo),
			zap.Uint32("ServerId", msg.ServerId),
			zap.ByteString("MSG", msg.Data),
			zap.Int32("RequestId", int32(msg.RequestId)),
		)
		return
	}
	// 看Params 3 是不是有数据来判断是否不是走压缩版本
	d := sendNotify.Params[3].(string)
	if d == "" {
		key := sendNotify.Params[1].(string)
		d, e := r.RedisCache.GetStr("key:" + key)
		if e != nil {
			zap.L().Error(
				"SendNotify Data Check Error",
				zap.Uint32("ReqNo", msg.ReqNo),
				zap.Uint32("ServerId", msg.ServerId),
				zap.ByteString("MSG", msg.Data),
				zap.Error(e),
				zap.Int32("RequestId", int32(msg.RequestId)),
			)
			return
		}
		commonNotify := new(JSONRPCRequest)
		err = json.Unmarshal([]byte(d), &commonNotify)
		if err != nil {
			zap.L().Error(
				"commonNotify Unmarshal Error",
				zap.Uint32("ReqNo", msg.ReqNo),
				zap.Uint32("ServerId", msg.ServerId),
				zap.Error(err),
				zap.Int32("RequestId", int32(msg.RequestId)),
			)
			return
		}
		if len(commonNotify.Params) == 0 {
			zap.L().Error(
				"CommonNotify Params Length Error",
				zap.Any("CommonNotify", commonNotify),
				zap.String("ReqNo", ss.reqNo),
			)
			return
		}
		// 拼装数据，发送给矿机
		sendNotify.Params[1] = commonNotify.Params[1]
		sendNotify.Params[3] = commonNotify.Params[3]
		sendNotify.Params[4] = commonNotify.Params[4]
		sendNotify.Params[7] = commonNotify.Params[7]
	}

	sd, err := json.Marshal(sendNotify)
	if err != nil {
		zap.L().Error(
			"SendNotify Marshal Error",
			zap.Uint32("ReqNo", msg.ReqNo),
			zap.Uint32("ServerId", msg.ServerId),
			zap.ByteString("MSG", msg.Data),
			zap.Error(err),
			zap.Int32("RequestId", int32(msg.RequestId)),
		)
		return
	} else {
		zap.L().Debug(
			"SendNotifyData",
			zap.Uint32("ReqNo", msg.ReqNo),
			zap.Uint32("ServerId", msg.ServerId),
			zap.ByteString("SendNotifyData", sd),
			zap.Int32("RequestId", int32(msg.RequestId)),
		)
	}
	if ss != nil && ss.minerConn != nil {
		ss.Write2Miner(append(sd, byte('\n')))
	}

}

func (r *RAgentClient) setDiff(ss *StratumSession, msg *ServerRequest) {
	zap.L().Debug(
		"SetDiff",
		zap.Uint32("ReqNo", msg.ReqNo),
		zap.Uint32("ServerId", msg.ServerId),
		zap.Any("MSG", msg),
		zap.Int32("RequestId", int32(msg.RequestId)),
	)
	var (
		diff uint32
	)
	diff = binary.LittleEndian.Uint32(msg.Data[0:4])
	req := JSONRPCRequest{ID: msg.RequestId, Method: "mining.set_difficulty"}
	req.SetParam(diff)
	buf, err := req.ToJSONBytes()
	if err != nil {
		return
	}
	ss.Write2Miner(append(buf, byte('\n')))
}

func (r RAgentClient) MinerOffline(ss *StratumSession) {
	ss.MinerClose()
}

func (r RAgentClient) MinerOffline2Server(ss *StratumSession) {
	ss.handleMinerOffline()
}

func (r *RAgentClient) AutoTransServer(isAutoTransServer bool) {
	// 避免初次连接还未完成就开始切server地址
	time.Sleep(time.Second * 3)

	firstServerAddr := fmt.Sprintf("%s:%d", r.Config.Servers[0].Ip, r.Config.Servers[0].Port)
	// 死循环判断server是否需要切换重启
	for {
		// 配置自动切换server, 到切换时间时, 自动切换server
		if isAutoTransServer {
			if r.serverM.serverStartTime+cast.ToInt64(r.Config.AutoTransServerDual) <= time.Now().Unix() {
				r.serverM = r.NewClientManager()
				r.Sm = NewStratumSessionManager(Rac.Config.ServerId, r.serverM)
				zap.L().Info("Server already changed.", zap.Uint64("serverId: ", Rac.Config.ServerId))
			}
		} else if r.serverM.serverAddr != firstServerAddr {
			// 配置不自动切换server, 如果第一个server可以连通, 默认以第一个server为优先
			sm, err := r.TryFirstServe()
			if err == nil {
				r.serverM = sm
				r.Sm = NewStratumSessionManager(Rac.Config.ServerId, r.serverM)
				zap.L().Info("Server connect first.", zap.Uint64("serverId: ", Rac.Config.ServerId))
			}
			zap.L().Error("Try connect first server failed.", zap.Uint64("serverId: ", Rac.Config.ServerId))
		}

		// 未连接server或者连接断开时, 随机重连一个server
		if r.serverM == nil || !r.serverM.isConn {
			r.serverM = Rac.NewClientManager()
			r.Sm = NewStratumSessionManager(Rac.Config.ServerId, r.serverM)
			zap.L().Info("Server connected.", zap.Uint64("serverId: ", Rac.Config.ServerId))
		}

		time.Sleep(time.Second)
	}

}

// 通用下发接口
func (r *RAgentClient) miningNotifyCommon(ss *StratumSession, msg *ServerRequest) {
	st := time.Now()
	zap.L().Debug("miningNotifyCommon Start", zap.Uint32("ReqNo:", msg.ReqNo), zap.ByteString("Data:", msg.Data), zap.String("time", st.String()))
	commonNotify := new(JSONRPCRequest)
	err := json.Unmarshal(msg.Data, &commonNotify)
	if err != nil {
		zap.L().Info("MiningNotifyCommon Error", zap.Error(err))
		return
	}
	md5OriStr := commonNotify.Params[1].(string) + commonNotify.Params[3].(string) + fmt.Sprintf("%s", commonNotify.Params[4]) + commonNotify.Params[7].(string)
	//keyStr := fmt.Sprintf("%x", md5.Sum([]byte(md5OriStr)))
	keyStr := fmt.Sprintf("%d %x", msg.ServerId, md5.Sum([]byte(md5OriStr)))
	_, err = r.RedisCache.GetStr(keyStr)
	if err.Error() == "redigo: nil returned" {
		zap.L().Debug("miningNotifyCommon set key", zap.Uint32("ReqNo:", msg.ReqNo), zap.ByteString("Data:", msg.Data), zap.String("key", keyStr))
		r.RedisCache.SetStr("key:"+keyStr, string(msg.Data), 3*60*1000)
	}
	zap.L().Debug("miningNotifyCommon End", zap.Uint32("ReqNo:", msg.ReqNo), zap.ByteString("Data:", msg.Data), zap.Int64("spend", st.Sub(time.Now()).Nanoseconds()))
}

type Server struct {
	ip   string
	port int
}

type ServerRequest struct {
	Magic       byte
	MessageType byte
	Length      uint16
	ServerId    uint32
	ReqNo       uint32
	RequestId   uint32
	Data        []byte
}
