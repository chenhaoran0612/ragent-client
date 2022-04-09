package kit

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync/atomic"

	"go.uber.org/zap"
)

var reqNo int64

func GetCommonReqNo() int64 { return atomic.AddInt64(&reqNo, 1) }

//Go 使用这个避免了协成panic导致程序退出
func Go(f func(v []interface{}), v ...interface{}) {
	go func(v []interface{}) {
		defer func() {
			if err := recover(); err != nil {
				zap.L().Error("GOROUTINE RECOVERED", zap.Any("Error Reason : ", err))
			}
		}()
		f(v)
	}(v)
}

func G(f func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				zap.L().Error("GOROUTINE RECOVERED", zap.Any("Error Reason : ", err))
			}
		}()
		f()
	}()
}

func IP2Long(ip string) uint32 {
	var long uint32
	binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.LittleEndian, &long)
	return long
}

func DeleteByteTail0(b []byte) []byte {
	spaceNum := 0
	for i := len(b) - 1; i >= 0; i-- { // 去除字符串尾部的所有空格
		if b[i] == 0 {
			spaceNum++
		} else {
			break
		}
	}
	return b[:len(b)-spaceNum]
}

func FindFirst0Index(b []byte) int {
	spaceNum := 0
	for i := 0; i <= len(b)-1; i++ {
		if b[i] != 0 {
			spaceNum++
		} else {
			break
		}
	}
	return spaceNum
}
