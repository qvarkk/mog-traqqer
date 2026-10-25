package logger

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/qvarkk/go-fiber-template/backend/internal/config"
)

const (
	fileMaxSizeMB  = 100
	fileMaxBackups = 5
	fileMaxAgeDays = 30
)

var Module = fx.Module("logger", fx.Provide(New))

func New(lc fx.Lifecycle, cfg *config.Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(string(cfg.Log.Level))
	if err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", cfg.Log.Level, err)
	}

	cores := []zapcore.Core{
		zapcore.NewCore(consoleEncoder(cfg.App.Env), zapcore.AddSync(os.Stdout), level),
	}

	var fileSink *lumberjack.Logger
	if cfg.Log.File != "" {
		fileSink = &lumberjack.Logger{
			Filename:   cfg.Log.File,
			MaxSize:    fileMaxSizeMB,
			MaxBackups: fileMaxBackups,
			MaxAge:     fileMaxAgeDays,
			Compress:   true,
		}

		fileEncoderConfig := zap.NewProductionEncoderConfig()
		fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(fileEncoderConfig),
			zapcore.AddSync(fileSink),
			level,
		))
	}

	log := zap.New(
		zapcore.NewTee(cores...),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			if fileSink != nil {
				_ = fileSink.Close()
			}
			_ = log.Sync()
			return nil
		},
	})

	return log, nil
}

// consoleEncoder returns the stdout encoder: a readable line in development,
// one JSON object per line otherwise, so log aggregators can parse it.
func consoleEncoder(env config.AppEnv) zapcore.Encoder {
	if !env.IsDevelopment() {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		return zapcore.NewJSONEncoder(encoderConfig)
	}

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.ConsoleSeparator = " | "

	return zapcore.NewConsoleEncoder(encoderConfig)
}
