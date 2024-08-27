package rotate

import (
	"fmt"
	"os"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 读取配置文件 获取 1. zapcore.EncoderConfig 2. logDir 3.  rotationTime 4.maxAge 5.rotationSize 6.levels  7. 控制台输出还是日志文件输出

// ZapCore 支持日志多等级文件输出
func ZapCore(config zapcore.EncoderConfig, isConsole bool, writes map[zapcore.Level]zapcore.WriteSyncer) []zapcore.Core {
	cores := make([]zapcore.Core, 0)
	var encoder zapcore.Encoder
	encoder = zapcore.NewConsoleEncoder(config)
	if !isConsole {
		encoder = zapcore.NewJSONEncoder(config)
	}
	for level, syncer := range writes {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.Lock(zapcore.AddSync(syncer)), levelEnablerFunc(level)))
	}
	return cores
}

// WriteSyncers 初始化输出文件
func WriteSyncers(logDir string, rotationTime, maxAge time.Duration, rotationSize int64, levels ...string) map[zapcore.Level]zapcore.WriteSyncer {

	if rotationTime <= 0 {
		rotationTime = 1 * time.Second
	}

	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}

	if rotationSize <= 0 {
		rotationSize = 1024 * 1024 * 3
	} else {
		rotationSize = rotationSize * 1024 * 1024
	}

	if logDir == "" {
		logDir = "."
	} else {
		logDir = strings.TrimRight(logDir, "/")
	}

	writes := make(map[zapcore.Level]zapcore.WriteSyncer)
	for i := 0; i < len(levels); i++ {
		level, err := zapcore.ParseLevel(levels[i])
		if err != nil {
			panic(err)
		}
		logPrefix := fmt.Sprintf("%s/%s", logDir, levels[i])
		rotate, err := rotatelogs.New(logPrefix+"%Y%m%d%H%M.log", rotatelogs.WithLinkName(logPrefix+".log"), rotatelogs.WithMaxAge(maxAge), rotatelogs.WithRotationTime(rotationTime), rotatelogs.WithRotationSize(rotationSize))
		if err != nil {
			panic(err)
		}
		writes[level] = zapcore.AddSync(rotate)
	}
	return writes
}

// ZapCoreConsole 支持控制台输出
func ZapCoreConsole(config zapcore.EncoderConfig, levels ...string) []zapcore.Core {
	cores := make([]zapcore.Core, 0)

	for i := 0; i < len(levels); i++ {
		level, err := zapcore.ParseLevel(levels[i])
		if err != nil {
			panic(err)
		}
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(config), zapcore.Lock(zapcore.AddSync(os.Stdout)), level))
	}
	return cores
}

// levelEnablerFunc 支持严格按照日志等级文件切割
func levelEnablerFunc(level zapcore.Level) zap.LevelEnablerFunc {
	switch level {
	case zapcore.DebugLevel:
		return func(level zapcore.Level) bool { // 调试级别
			return level == zap.DebugLevel
		}
	case zapcore.InfoLevel:
		return func(level zapcore.Level) bool { // 日志级别
			return level == zap.InfoLevel
		}
	case zapcore.WarnLevel:
		return func(level zapcore.Level) bool { // 警告级别
			return level == zap.WarnLevel
		}
	case zapcore.ErrorLevel:
		return func(level zapcore.Level) bool { // 错误级别
			return level == zap.ErrorLevel
		}
	case zapcore.DPanicLevel:
		return func(level zapcore.Level) bool { // dpanic级别
			return level == zap.DPanicLevel
		}
	case zapcore.PanicLevel:
		return func(level zapcore.Level) bool { // panic级别
			return level == zap.PanicLevel
		}
	case zapcore.FatalLevel:
		return func(level zapcore.Level) bool { // 终止级别
			return level == zap.FatalLevel
		}
	default:
		return func(level zapcore.Level) bool { // 调试级别
			return level == zap.DebugLevel
		}
	}
}
