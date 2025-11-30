package logger

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with consistent field structure
type Logger struct {
	*zap.Logger
}

// Fields for consistent structured logging
type Fields struct {
	Source        string
	Namespace     string
	EventType     string
	ObservationID string
	Severity      string
	Component     string
	Operation     string
	ResourceKind  string
	ResourceName  string
	Error         error
	CorrelationID string
	Duration      string
	Count         int
	Reason        string
	Message       string
	// Additional fields as key-value pairs
	Additional map[string]interface{}
}

var (
	// Global logger instance
	globalLogger *Logger
)

// Init initializes the global logger
func Init(level string, development bool) error {
	var zapLevel zapcore.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		zapLevel = zapcore.DebugLevel
	case "INFO":
		zapLevel = zapcore.InfoLevel
	case "WARN", "WARNING":
		zapLevel = zapcore.WarnLevel
	case "ERROR":
		zapLevel = zapcore.ErrorLevel
	case "FATAL":
		zapLevel = zapcore.FatalLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	var config zap.Config
	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.CallerKey = "caller"
	}

	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, err := config.Build(
		zap.AddCallerSkip(1), // Skip logger wrapper calls
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return err
	}

	globalLogger = &Logger{Logger: logger}
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// Fallback initialization
		_ = Init("INFO", false)
	}
	return globalLogger
}

// WithFields creates a new logger with structured fields
func (l *Logger) WithFields(fields Fields) *zap.Logger {
	zapFields := []zap.Field{}

	if fields.Source != "" {
		zapFields = append(zapFields, zap.String("source", fields.Source))
	}
	if fields.Namespace != "" {
		zapFields = append(zapFields, zap.String("namespace", fields.Namespace))
	}
	if fields.EventType != "" {
		zapFields = append(zapFields, zap.String("event_type", fields.EventType))
	}
	if fields.ObservationID != "" {
		zapFields = append(zapFields, zap.String("observation_id", fields.ObservationID))
	}
	if fields.Severity != "" {
		zapFields = append(zapFields, zap.String("severity", fields.Severity))
	}
	if fields.Component != "" {
		zapFields = append(zapFields, zap.String("component", fields.Component))
	}
	if fields.Operation != "" {
		zapFields = append(zapFields, zap.String("operation", fields.Operation))
	}
	if fields.ResourceKind != "" {
		zapFields = append(zapFields, zap.String("resource_kind", fields.ResourceKind))
	}
	if fields.ResourceName != "" {
		zapFields = append(zapFields, zap.String("resource_name", fields.ResourceName))
	}
	if fields.Error != nil {
		zapFields = append(zapFields, zap.Error(fields.Error))
	}
	if fields.CorrelationID != "" {
		zapFields = append(zapFields, zap.String("correlation_id", fields.CorrelationID))
	}
	if fields.Duration != "" {
		zapFields = append(zapFields, zap.String("duration", fields.Duration))
	}
	if fields.Count > 0 {
		zapFields = append(zapFields, zap.Int("count", fields.Count))
	}
	if fields.Reason != "" {
		zapFields = append(zapFields, zap.String("reason", fields.Reason))
	}
	if fields.Additional != nil {
		for k, v := range fields.Additional {
			zapFields = append(zapFields, zap.Any(k, v))
		}
	}

	return l.Logger.With(zapFields...)
}

// WithContext adds correlation ID from context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}

	correlationID := GetCorrelationID(ctx)
	if correlationID != "" {
		return &Logger{Logger: l.Logger.With(zap.String("correlation_id", correlationID))}
	}

	return l
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, fields ...Fields) {
	if len(fields) > 0 {
		l.WithFields(fields[0]).Debug(msg)
	} else {
		l.Logger.Debug(msg)
	}
}

// Info logs at info level
func (l *Logger) Info(msg string, fields ...Fields) {
	if len(fields) > 0 {
		l.WithFields(fields[0]).Info(msg)
	} else {
		l.Logger.Info(msg)
	}
}

// Warn logs at warn level
func (l *Logger) Warn(msg string, fields ...Fields) {
	if len(fields) > 0 {
		l.WithFields(fields[0]).Warn(msg)
	} else {
		l.Logger.Warn(msg)
	}
}

// Error logs at error level
func (l *Logger) Error(msg string, fields ...Fields) {
	if len(fields) > 0 {
		l.WithFields(fields[0]).Error(msg)
	} else {
		l.Logger.Error(msg)
	}
}

// Fatal logs at fatal level and exits
func (l *Logger) Fatal(msg string, fields ...Fields) {
	if len(fields) > 0 {
		l.WithFields(fields[0]).Fatal(msg)
	} else {
		l.Logger.Fatal(msg)
	}
}

// Context key for correlation ID
type contextKey string

const correlationIDKey contextKey = "correlation_id"

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// GetCorrelationID extracts correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// Convenience functions for global logger
func Debug(msg string, fields ...Fields) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...Fields) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...Fields) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...Fields) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...Fields) {
	GetLogger().Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
