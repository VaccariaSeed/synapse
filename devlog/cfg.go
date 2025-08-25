package devlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	defaultName = "default"
	defaultLog  = "default_log"
)

var defaultLogger *LogModel
var lc *LogConfig
var lcLock sync.Mutex

func LoadConfig(lci *LogConfig) error {
	lcLock.Lock()
	defer lcLock.Unlock()
	if lc != nil {
		return nil
	}
	lc = lci
	if lc == nil {
		lc = &LogConfig{}
	}
	if strings.TrimSpace(lc.Level) == "" {
		lc.Level = "debug"
	}
	if strings.TrimSpace(lc.FilePath) == "" {
		lc.FilePath = "./vs_log"
	}
	if lc.MaxSize <= 0 {
		lc.MaxSize = 10
	}
	if lc.MaxBackups <= 0 {
		lc.MaxBackups = 10
	}
	if lc.MaxAge <= 0 {
		lc.MaxAge = 7
	}
	//创建一个默认的日志轮转
	defaultLogger = &LogModel{
		DeviceName: defaultLog,
		DeviceId:   -1,
		DeviceType: defaultName,
	}
	sugarLogger, err := createLogConf(defaultLog)
	if err != nil {
		return err
	}
	defaultLogger.SugaredLogger = sugarLogger
	return nil
}

type LogConfig struct {
	Level      string `json:"level"`       // 日志级别: debug, info, warn, error, dpanic, panic, fatal
	FilePath   string `json:"filePath"`    // 日志文件路径
	MaxSize    int    `json:"max_size"`    // 每个日志文件的最大尺寸(单位：MB)
	MaxBackups int    `json:"max_backups"` // 日志文件最多保存多少个备份
	MaxAge     int    `json:"max_age"`     // 文件最多保存多少天
	Compress   bool   `json:"compress"`    // 是否压缩
}

func createLogConf(device string) (*zap.SugaredLogger, error) {
	if lc == nil {
		err := LoadConfig(nil)
		if err != nil {
			return nil, err
		}
	}
	level := getZapLevel(lc.Level)
	// 文件核心（带轮转）
	fileCore, err := newFileCore(level, device)
	if err != nil {
		return nil, err
	}
	cores := []zapcore.Core{fileCore, newConsoleCore(level)}
	// 创建tee核心
	core := zapcore.NewTee(cores...)
	// 创建logger，添加调用者信息和堆栈跟踪
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel), // 错误级别以上添加堆栈跟踪
	)
	// 创建SugaredLogger
	return logger.Sugar(), nil
}

func newConsoleCore(level zapcore.LevelEnabler) zapcore.Core {
	consoleEncoder := getConsoleEncoder()
	return zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)
}

func getConsoleEncoder() zapcore.Encoder {
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

func newFileCore(level zapcore.LevelEnabler, device string) (zapcore.Core, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(lc.FilePath, 0755); err != nil {
		return nil, err
	}

	// 配置lumberjack日志轮转
	lumberJackLogger := &lumberjack.Logger{
		Filename:   path.Join(lc.FilePath, device+".log"),
		MaxSize:    lc.MaxSize,    // 单位：MB
		MaxBackups: lc.MaxBackups, // 最大备份数量
		MaxAge:     lc.MaxAge,     // 单位：天
		Compress:   lc.Compress,   // 是否压缩
		LocalTime:  true,          // 使用本地时间
	}

	// 文件编码器 - 使用控制台格式
	fileEncoder := getCommonEncoder()
	return zapcore.NewCore(fileEncoder, zapcore.AddSync(lumberJackLogger), level), nil
}

func getCommonEncoder() zapcore.Encoder {
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
}

func getZapLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.DebugLevel
	}
}
