package logger

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const LevelSuccess zapcore.Level = zapcore.Level(100)

type zapLogger struct {
	l *zap.Logger
}

// NewZapLogger crea
// tes a zap-backed logger (prod or dev).
func NewZapLogger(env string) (Logger, error) {
	var base *zap.Logger
	var err error

	if env == "dev" {
		fmt.Printf("Creating Dev logger \n\n")
		cfg := zap.NewDevelopmentConfig()
		cfg.DisableCaller = true
		base, err = cfg.Build()
	} else {
		fmt.Printf("Creating Prod logger \n\n")
		base, err = zap.NewProduction()
	}
	if err != nil {
		return nil, err
	}

	return &zapLogger{l: base}, nil
}

func (z *zapLogger) Info(msg string, fields ...Field) {
	z.l.Info(msg, toZapFields(fields)...)
}
func (z *zapLogger) Success(msg string, fields ...Field) {
	z.l.Info(color.GreenString(" SUCCESS ✅" + " " + msg))
}

func (z *zapLogger) Error(msg string, fields ...Field) {

	z.l.Error(color.RedString(msg), toZapFields(fields)...)
}
func (z *zapLogger) Debug(msg string, fields ...Field) {
	z.l.Debug(color.CyanString(msg), toZapFields(fields)...)
}
func (z *zapLogger) Warn(msg string, fields ...Field) {
	z.l.Warn(color.YellowString(msg), toZapFields(fields)...)
}
func (z *zapLogger) Fatal(msg string, fields ...Field) {
	z.Error(msg, fields...)
	os.Exit(1)
}

func (z *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{l: z.l.With(toZapFields(fields)...)}
}
func (z *zapLogger) Flush() {
	z.l.Sync()
}

func (z *zapLogger) WithContext(ctx context.Context) Logger {
	// You can enrich logger from context here (e.g. trace ID).
	return z
}

// Convert our Field to zap.Field
func toZapFields(fields []Field) []zap.Field {
	zfs := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		switch v := f.Value.(type) {
		case string:
			zfs = append(zfs, zap.String(f.Key, v))
		case int:
			zfs = append(zfs, zap.Int(f.Key, v))
		case int64:
			zfs = append(zfs, zap.Int64(f.Key, v))
		case float64:
			zfs = append(zfs, zap.Float64(f.Key, v))
		case bool:
			zfs = append(zfs, zap.Bool(f.Key, v))
		default:
			zfs = append(zfs, zap.Any(f.Key, v))
		}
	}
	return zfs
}
