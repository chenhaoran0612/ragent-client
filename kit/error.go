package kit

import "errors"

type RagentServerError struct {
	// 错误号
	ErrNo int
	// 错误信息
	ErrMsg string
}

//  NewRangentServerError
func NewRagentServerError(errNo int, errMsg string) *RagentServerError {
	err := new(RagentServerError)
	err.ErrNo = errNo
	err.ErrMsg = errMsg
	return err
}

// Error 实现StratumError的Error()接口以便其被当做error类型使用
func (err *RagentServerError) Error() string {
	return err.ErrMsg
}

// ToJSONRPCArray 转换为JSONRPCArray
func (err *RagentServerError) ToJSONRPCArray() JSONRPCArray {
	if err == nil {
		return nil
	}

	return JSONRPCArray{err.ErrNo, err.ErrMsg, nil}
}

// JSONRPCArray JSON RPC 数组
type JSONRPCArray []interface{}

var (
	// ErrBufIOReadTimeout 从bufio.Reader中读取数据时超时
	ErrBufIOReadTimeout = errors.New("BufIO Read Timeout")

	// ErrParseSubscribeResponseFailed 解析订阅响应失败
	ErrParseSubscribeResponseFailed = errors.New("Parse Subscribe Response Failed")

	// ErrSessionIDInconformity 返回的会话ID和当前保存的不匹配
	ErrSessionIDInconformity = errors.New("Session ID Inconformity")

	// ErrAuthorizeFailed 认证失败
	ErrAuthorizeFailed = errors.New("Authorize Failed")
)

var (
	StratumErrJobNotFound    = NewRagentServerError(21, "Job not found (=stale)")
	StratumErrDuplicateShare = NewRagentServerError(22, "Duplicate share")
	StratumErrLowDifficulty  = NewRagentServerError(23, "Low difficulty")
	StratumErrTimeTooOld     = NewRagentServerError(31, "Time too old")
	StratumErrTimeTooNew     = NewRagentServerError(32, "Time too new")

	StratumErrNeedSubscribed         = NewRagentServerError(101, "Need Subscribed")
	StratumErrNeedAuthorize          = NewRagentServerError(102, "Need Authorize")
	StratumErrTooFewParams           = NewRagentServerError(103, "Too Few Params")
	StratumErrWorkerNameMustBeString = NewRagentServerError(104, "Worker Name Must be a String")
	StratumErrWorkerNameStartWrong   = NewRagentServerError(105, "Worker Name Cannot Start with '.'")
	StratumErrAuthorizeFailed        = NewRagentServerError(106, "Authorized Failed, Try Again")
	StratumErrMinerMustBeString      = NewRagentServerError(107, "Miner Must Be String")

	StratumErrStratumServerNotFound      = NewRagentServerError(301, "Stratum Server Not Found")
	StratumErrConnectStratumServerFailed = NewRagentServerError(302, "Connect Stratum Server Failed")
	StratumErrResourceLimit              = NewRagentServerError(303, "Max Workers Count Reach Limit")
)
