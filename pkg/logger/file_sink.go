package logger

import (
	"os"
	"path/filepath"
	"regexp"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewZapLoggerWithFile is NewZapLogger plus a second sink: JSON lines in path (rotated at 50 MB, 3 backups),
// each tagged with service. The console output is unchanged; the file is what Grafana Alloy ships to Loki.
// An empty path means no file sink.
func NewZapLoggerWithFile(env, service, path string) (Logger, error) {
	l, err := NewZapLogger(env)
	if err != nil || path == "" {
		return l, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	level := zapcore.InfoLevel
	if env == "dev" {
		level = zapcore.DebugLevel
	}
	file := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(&lumberjack.Logger{Filename: path, MaxSize: 50, MaxBackups: 3}),
		level,
	)
	file = stripANSICore{file.With([]zap.Field{zap.String("service", service)})}

	base := l.(*zapLogger).l
	return &zapLogger{l: base.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, file)
	}))}, nil
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSICore drops the terminal colour codes the console methods add to messages, so they don't land in Loki.
type stripANSICore struct{ zapcore.Core }

func (s stripANSICore) With(fields []zapcore.Field) zapcore.Core {
	return stripANSICore{s.Core.With(fields)}
}

func (s stripANSICore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(e.Level) {
		return ce.AddCore(e, s)
	}
	return ce
}

func (s stripANSICore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	e.Message = ansiEscape.ReplaceAllString(e.Message, "")
	return s.Core.Write(e, fields)
}
