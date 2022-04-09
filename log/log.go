package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ZapLog struct {
	Filename    string
	MaxSize     int
	MaxBackups  int
	MaxAge      int
	AtomicLevel *zap.AtomicLevel
	Logger      *zap.Logger
	Verbose     int
}

var gZapLog *ZapLog

func NewZapLog(filename string, maxSize int, maxBackups int, maxAge int, level string, verbose int, compress bool, initField []zap.Field) *ZapLog {
	if gZapLog != nil {
		return gZapLog
	}

	zapLevel := zap.NewAtomicLevel()
	// 设置日志级别
	setLevel(&zapLevel, level)

	gZapLog = new(ZapLog)
	gZapLog.Filename = filename
	gZapLog.MaxSize = maxSize
	gZapLog.MaxBackups = maxBackups
	gZapLog.MaxAge = maxSize
	gZapLog.AtomicLevel = &zapLevel
	gZapLog.Verbose = verbose

	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   gZapLog.Filename,
		MaxSize:    gZapLog.MaxSize, // megabytes
		MaxBackups: gZapLog.MaxBackups,
		MaxAge:     gZapLog.MaxAge, // days
		LocalTime:  true,
		Compress:   compress,
	})

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "@timestamp",
		LevelKey:       "L",
		NameKey:        "logger",
		CallerKey:      "C",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,  // 小写编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,     // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder, //
		EncodeCaller:   zapcore.ShortCallerEncoder,     // 短路径编码器
		EncodeName:     zapcore.FullNameEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		w,
		gZapLog.AtomicLevel,
	)
	core = core.With(initField)

	// 开启开发模式，堆栈跟踪
	caller := zap.AddCaller()
	// 开启文件及行号
	development := zap.Development()

	logger := zap.New(core, caller, development)
	gZapLog.Logger = logger

	return gZapLog
}

func setLevel(zapLevel *zap.AtomicLevel, level string) {
	switch level {
	case "info":
		zapLevel.SetLevel(zap.InfoLevel)
		break
	case "warn":
		zapLevel.SetLevel(zap.WarnLevel)
		break
	case "error":
		zapLevel.SetLevel(zap.ErrorLevel)
		break
	case "debug":
		zapLevel.SetLevel(zap.DebugLevel)
		break
	case "fatal":
		zapLevel.SetLevel(zap.FatalLevel)
		break
	default:
		zapLevel.SetLevel(zap.WarnLevel)
	}
}
