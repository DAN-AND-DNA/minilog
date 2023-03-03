package minilog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"os"
	"runtime/debug"
)

var (
	// 默认值
	defaultConfig = Config{
		Environment: "development",
		Filename:    "./tmp.log",
		MaxSize:     500,
		MaxBackups:  3,
		MaxAge:      10,
	}
)

type MiniLog struct {
	*zap.Logger
}

type Config struct {
	Environment string // 是否生产环境 (development or production)
	Filename    string // 日志文件路径
	MaxSize     int    // 日志文件的大小（mb）
	MaxBackups  int    // 老日志最大保持数，不填不删
	MaxAge      int    // 老日志最大保持日期（天）,不填不删
}

func (logger *MiniLog) GetZap() *zap.Logger {
	if logger == nil {
		return nil
	}

	return logger.Logger
}

// applyDefault 设置默认值
func applyDefault(config Config) Config {
	if config.Filename == "" {
		config.Filename = defaultConfig.Filename
	}

	if config.MaxSize <= 0 {
		config.MaxSize = defaultConfig.MaxSize
	}

	if config.MaxBackups <= 0 {
		config.MaxBackups = defaultConfig.MaxBackups
	}

	if config.MaxAge <= 0 {
		config.MaxAge = defaultConfig.MaxAge
	}

	return config
}

// FIXME nopSyncer 避免标准输出输入sync时报错
type nopSyncer struct {
	file *os.File
}

func (nop nopSyncer) Write(p []byte) (n int, err error) {
	return nop.file.Write(p)
}
func (nop nopSyncer) Sync() error {
	return nil
}

func New(configArg ...Config) *MiniLog {
	// 处理配置的默认值
	var config = defaultConfig
	if len(configArg) != 0 {
		config = configArg[0]
		config = applyDefault(config)
	}

	logger := &MiniLog{}

	var cores []zapcore.Core
	var options []zap.Option

	// 日志轮转
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   config.Filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
	})

	if config.Environment == "production" {
		// 生产环境
		jsonEncoderConfig := zap.NewProductionEncoderConfig()
		// 自定义格式
		jsonEncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		jsonEncoderConfig.EncodeCaller = zapcore.FullCallerEncoder

		jsonEncoder := zapcore.NewJSONEncoder(jsonEncoderConfig)

		enabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.ErrorLevel
		})

		// 输出到文件
		cores = append(cores, zapcore.NewCore(jsonEncoder, fileWriter, enabler))
		// TODO kafka

		options = append(options, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		// 默认都是开发环境
		jsonEncoderConfig := zap.NewDevelopmentEncoderConfig()
		// 自定义格式
		jsonEncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		jsonEncoderConfig.EncodeCaller = zapcore.FullCallerEncoder

		jsonEncoder := zapcore.NewJSONEncoder(jsonEncoderConfig)
		consoleEncoder := zapcore.NewConsoleEncoder(jsonEncoderConfig)

		enabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.DebugLevel
		})

		consoleWriter := zapcore.Lock(nopSyncer{os.Stderr})

		// 输出到控制台
		cores = append(cores, zapcore.NewCore(consoleEncoder, consoleWriter, enabler))
		// 输出到文件
		cores = append(cores, zapcore.NewCore(jsonEncoder, fileWriter, enabler))

		options = append(options, zap.AddCaller(), zap.Development(), zap.AddStacktrace(zapcore.ErrorLevel))
	}

	logger.Logger = zap.New(zapcore.NewTee(cores...)).WithOptions(options...)

	return logger
}

func (logger *MiniLog) Close() {
	if logger == nil {
		return
	}

	if err := logger.Sync(); err != nil {
		log.Println(err, string(debug.Stack()))
	}
}
