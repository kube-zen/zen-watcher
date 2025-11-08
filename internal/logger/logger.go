package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	CRIT
)

var (
	currentLevel = INFO
	prefix       = ""
)

func init() {
	log.SetFlags(0)
	
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		switch strings.ToUpper(levelStr) {
		case "DEBUG":
			currentLevel = DEBUG
		case "INFO":
			currentLevel = INFO
		case "WARN", "WARNING":
			currentLevel = WARN
		case "ERROR":
			currentLevel = ERROR
		case "CRIT", "CRITICAL":
			currentLevel = CRIT
		}
	}
	
	if p := os.Getenv("LOG_PREFIX"); p != "" {
		prefix = p + " "
	} else {
		prefix = "zen-watcher "
	}
}

func Debug(format string, args ...interface{}) {
	logf(DEBUG, format, args...)
}

func Info(format string, args ...interface{}) {
	logf(INFO, format, args...)
}

func Warn(format string, args ...interface{}) {
	logf(WARN, format, args...)
}

func Error(format string, args ...interface{}) {
	logf(ERROR, format, args...)
}

func Crit(format string, args ...interface{}) {
	logf(CRIT, format, args...)
}

func logf(level Level, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}
	
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	levelStr := levelString(level)
	message := fmt.Sprintf(format, args...)
	
	log.Printf("%s [%s] %s%s", timestamp, levelStr, prefix, message)
}

func levelString(level Level) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO "
	case WARN:
		return "WARN "
	case ERROR:
		return "ERROR"
	case CRIT:
		return "CRIT "
	default:
		return "???? "
	}
}
