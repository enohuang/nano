package zaplog

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"gnano/pkg/zaplog/rotate"
)

// New
// config.toml
// [zap]
// #prod 或者 dev
// model="prod"
// #日志文件存放文件夹
// log_dir="logs"
// #日志跟踪key
// stacktrace_key="gate"
// #1.capital 2.capitalColor  3.color
// encode_level="capital"
// # 1.console  输出到控制台   2.空值 为输出到日志文件
// out=""
// # 1. json  2.console
// format="json"
//
//	[rotate]
//
// # 文件切割时间间隔
//
//	rotation_time="1h"
//
// # 文件最大存放时长
//
//	max_age="1d"
//
// # 文件大小 m
//
//	rotation_size=3
var (
	ZLog *zap.Logger
)

func Init() {
	levels := viper.GetStringSlice("zap.levels")
	if len(levels) == 0 {
		levels = allLevel()
	}
	var cores []zapcore.Core
	if isConsole() {
		cores = rotate.ZapCoreConsole(EncoderConfig(), levels...)
	} else {
		m := rotate.WriteSyncers(viper.GetString("zap.rotate.log_dir"), viper.GetDuration("zap.rotate.rotation_time"), viper.GetDuration("zap.rotate.max_age"), viper.GetInt64("zap.rotate.rotation_size"), levels...)
		switch format() {
		case "json":
			cores = rotate.ZapCore(EncoderConfig(), false, m)
		case "console":
			cores = rotate.ZapCore(EncoderConfig(), true, m)
		}
	}
	ZLog = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func New() *zap.Logger {
	Init()
	return ZLog
}

func UpdateLoggerConfig(zLog *zap.Logger) *zap.Logger {
	levels := viper.GetStringSlice("zap.levels")
	if len(levels) == 0 {
		levels = allLevel()
	}
	var cores []zapcore.Core
	if isConsole() {
		cores = rotate.ZapCoreConsole(EncoderConfig(), levels...)
	} else {
		m := rotate.WriteSyncers(viper.GetString("zap.rotate.log_dir"), viper.GetDuration("zap.rotate.rotation_time"), viper.GetDuration("zap.rotate.max_age"), viper.GetInt64("zap.rotate.rotation_size"), levels...)
		switch format() {
		case "json":
			cores = rotate.ZapCore(EncoderConfig(), false, m)
		case "console":
			cores = rotate.ZapCore(EncoderConfig(), true, m)
		}
	}
	options := []zap.Option{}
	for _, v := range cores {
		opt := zap.WrapCore(func(zapcore.Core) zapcore.Core {
			return v
		})
		options = append(options, opt)
	}

	if zLog == nil {
		zLog = ZLog
	}
	ZLog = zLog.WithOptions(options...)
	return ZLog
}

// EncoderConfig 读取配置文件 获取 1. zapcore.EncoderConfig 2. logDir 3.  rotationTime 4.maxAge 5.rotationSize 6.levels  7. 控制台输出还是日志文件输出
func EncoderConfig() zapcore.EncoderConfig {
	model := viper.GetString("zap.model")
	encoderConf := zap.NewDevelopmentEncoderConfig()
	switch model {
	case "prod":
		encoderConf = zap.NewProductionEncoderConfig()
	}
	messageKey := viper.GetString("zap.message_key")
	if messageKey != "" {
		encoderConf.MessageKey = messageKey
	}
	levelKey := viper.GetString("zap.level_key")
	if levelKey != "" {
		encoderConf.LevelKey = levelKey
	}
	stacktraceKey := viper.GetString("zap.stacktrace_key")
	if stacktraceKey != "" {
		encoderConf.StacktraceKey = stacktraceKey
	}
	encodeLevel := viper.GetString("zap.encode_level")
	levelEncoder := new(zapcore.LevelEncoder)
	err := levelEncoder.UnmarshalText([]byte(encodeLevel))
	if err != nil {
		panic(err)
	}
	encoderConf.EncodeLevel = *levelEncoder
	// 暂时不支持时间格式转换
	encoderConf.EncodeTime = zapcore.ISO8601TimeEncoder
	return encoderConf
}

func allLevel() []string {
	return []string{"debug", "info", "warn", "error", "dpanic", "panic", "fatal"}
}

// isConsole 输出位置
func isConsole() bool {
	return viper.GetString("zap.out") == "console"
}

// format  输出格式
func format() string {
	return viper.GetString("zap.format")
}
