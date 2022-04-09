package service

import (
	"amenity.com/ragent-client/service/client"
	"encoding/json"
	"fmt"
	"go.uber.org/zap/zapcore"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type RpcService struct {
	startTime int64
	Port      int
}

type RpcResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var Version = "V1.2"

func NewRpcService(port int) *RpcService {

	rpcService := new(RpcService)
	rpcService.Port = port

	return rpcService
}

func (rpcService *RpcService) Run() {

	rpcService.startTime = time.Now().Unix()

	zap.L().Info("listen http", zap.Int("rpcPort", rpcService.Port))

	http.HandleFunc("/slb/check", rpcService.SlbCheck)

	http.HandleFunc("/status", rpcService.Status)

	http.HandleFunc("/handle/level", rpcService.HandleLevel)

	err := http.ListenAndServe(fmt.Sprintf(":%d", rpcService.Port), nil)

	zap.L().Debug("rpcHttpd exited. switch service is disabled", zap.Error(err))

}

func (rpcService *RpcService) SlbCheck(w http.ResponseWriter, r *http.Request) {

	OKRpcResponse().seriallizeTo(w)
}

func (rpcService *RpcService) HandleLevel(w http.ResponseWriter, r *http.Request) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	type payload struct {
		Level   *zapcore.Level `json:"level"`
		Verbose int            `json:"verbose"`
	}

	resp := OKRpcResponse()

	switch r.Method {

	case http.MethodGet:
		current := client.Rac.ZapLog.AtomicLevel.Level()
		currenVerbose := client.Rac.ZapLog.Verbose
		resp.Data = payload{Level: &current, Verbose: currenVerbose}
		resp.seriallizeTo(w)
	case http.MethodPut:
		var req payload

		if errmess := func() string {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return fmt.Sprintf("Request body must be well-formed JSON: %v", err)
			}
			if req.Level == nil {
				return "Must specify a logging level."
			}
			return ""
		}(); errmess != "" {
			w.WriteHeader(http.StatusBadRequest)
			resp.Data = errorResponse{Error: errmess}
			resp.seriallizeTo(w)
			return
		}

		client.Rac.ZapLog.AtomicLevel.SetLevel(*req.Level)
		client.Rac.ZapLog.Verbose = req.Verbose
		resp.Data = req
		resp.seriallizeTo(w)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		resp.Data = errorResponse{
			Error: "Only GET and PUT are supported.",
		}
		resp.seriallizeTo(w)
	}
}

func OKRpcResponse() *RpcResponse {
	resp := new(RpcResponse)
	resp.Code = 0
	resp.Message = "OK"
	return resp
}

func (resp *RpcResponse) seriallizeTo(w io.Writer) error {

	respBytes, err := json.Marshal(*resp)

	if err != nil {
		return err
	}

	_, err = w.Write(respBytes)

	return err
}

type AvgValue struct {
	H1m  string `json:"1m"`
	H5m  string `json:"5m"`
	H15m string `json:"15m"`
	H1h  string `json:"1h"`
}
type HashRateStatus struct {
	HashRate   AvgValue `json:"hashRate"`
	RejectRate AvgValue `json:"rejectRate"`
}
type Data struct {
	Uptime  int64  `json:"uptime"`
	Version string `json:"version"`
	//Stat           *client.Stat     `json:"stat"`
	HashRateStatus *HashRateStatus `json:"hashRateStatus"`
}

func (rpcService *RpcService) Status(w http.ResponseWriter, r *http.Request) {

	zap.L().Info("info")
	zap.L().Warn("warn")
	zap.L().Error("error")
	zap.L().Debug("debug")

	if client.Rac == nil {
		resp := OKRpcResponse()
		resp.Data = nil
		resp.seriallizeTo(w)
		return
	}

	//hashRateMap, rejectRateMap, _ := client.Ras.Am.StatSvc.GetHashRate()
	//hashRateStatus := &HashRateStatus{
	//	AvgValue{
	//		readableHashRate(hashRateMap["1m"], true),
	//		readableHashRate(hashRateMap["5m"], true),
	//		readableHashRate(hashRateMap["15m"], true),
	//		readableHashRate(hashRateMap["1h"], true),
	//	},
	//	AvgValue{
	//		readableRejectRate(rejectRateMap["1m"]),
	//		readableRejectRate(rejectRateMap["5m"]),
	//		readableRejectRate(rejectRateMap["15m"]),
	//		readableRejectRate(rejectRateMap["1h"]),
	//	},
	//}

	data := Data{
		Uptime:  time.Now().Unix() - rpcService.startTime,
		Version: Version}

	resp := OKRpcResponse()
	resp.Data = data
	resp.seriallizeTo(w)
}

func readableRejectRate(rejectRate float64) string {

	return fmt.Sprintf("%.3f%s", rejectRate*100, "%")
}

func readableHashRate(hashRate *big.Int, withUnit bool) string {

	conf := [][]string{
		[]string{"18", "E"},
		[]string{"15", "P"},
		[]string{"12", "T"},
		[]string{"9", "G"},
		[]string{"6", "M"},
		[]string{"3", "K"},
		[]string{"0", ""},
	}

	for _, one := range conf {
		decimals, _ := strconv.Atoi(one[0])
		standard := new(big.Int).SetUint64(uint64(math.Pow(10, float64(decimals))))
		unit := one[1]
		if hashRate.Cmp(standard) > 0 {
			newHashRate := hashRate.Div(hashRate, standard.Div(standard, new(big.Int).SetUint64(1000)))
			floatHashRate := float64(newHashRate.Uint64()) / 1000
			result := fmt.Sprintf("%.3f", floatHashRate)
			if withUnit {
				result = fmt.Sprintf("%s %sH/s", result, unit)
			}
			return result
		}
	}

	result := "0"
	if withUnit {
		result = fmt.Sprintf("%s %sH/s", result, "")
	}

	return result
}
