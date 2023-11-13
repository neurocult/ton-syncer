package logger

import (
	"os"

	"github.com/eqtlab/ton-syncer/pkg/logger/output"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const logStreamLimit = 5

type Logger struct {
	*zap.Logger
	ch output.LimitedChanWriter
}

func New(debug bool) *Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	ch := output.NewLimitedChanWriter(logStreamLimit)

	consoleEncoder := zapcore.NewConsoleEncoder(config)
	defaultEncoder := consoleEncoder

	defaultLogLevel := zapcore.DebugLevel

	if !debug {
		defaultLogLevel = zapcore.InfoLevel
		defaultEncoder = zapcore.NewJSONEncoder(config)
	}

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(ch), defaultLogLevel),
		zapcore.NewCore(defaultEncoder, zapcore.AddSync(os.Stdout), defaultLogLevel),
	)

	return &Logger{
		Logger: zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)),
		ch:     ch,
	}
}

func (l *Logger) SubscriptionChannel() chan string {
	return l.ch
}
