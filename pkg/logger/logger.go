package logger

// Field is a generic key/value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// Logger is the interface your code depends on.
// It hides the actual logging implementation (Zap, Slog, etc.)
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Success(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	Flush()
}

// --- Helper functions for typed fields ---

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
}
