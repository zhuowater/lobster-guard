// logger.go — 结构化日志 Logger（v5.0 可观测性）
// 支持 text（默认）和 json 两种输出格式
// 零外部依赖，基于标准库实现
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelInfo  LogLevel = 0
	LogLevelWarn  LogLevel = 1
	LogLevelError LogLevel = 2
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelWarn:
		return "warn"
	case LogLevelError:
		return "error"
	default:
		return "info"
	}
}

// Logger 结构化日志记录器
// 支持 text 和 json 两种输出格式
type Logger struct {
	mu     sync.Mutex
	format string // "text" or "json"
	writer io.Writer
}

// NewLogger 创建新的 Logger 实例
// format: "text"（默认，传统 log.Printf 风格）或 "json"（JSON 行）
func NewLogger(format string, writer io.Writer) *Logger {
	if format != "json" {
		format = "text"
	}
	if writer == nil {
		writer = os.Stderr
	}
	return &Logger{
		format: format,
		writer: writer,
	}
}

// Info 输出 info 级别日志
func (l *Logger) Info(module, msg string, kvs ...interface{}) {
	l.log(LogLevelInfo, module, msg, kvs...)
}

// Warn 输出 warn 级别日志
func (l *Logger) Warn(module, msg string, kvs ...interface{}) {
	l.log(LogLevelWarn, module, msg, kvs...)
}

// Error 输出 error 级别日志
func (l *Logger) Error(module, msg string, kvs ...interface{}) {
	l.log(LogLevelError, module, msg, kvs...)
}

// log 内部日志输出
func (l *Logger) log(level LogLevel, module, msg string, kvs ...interface{}) {
	if l.format == "json" {
		l.logJSON(level, module, msg, kvs...)
	} else {
		l.logText(level, module, msg, kvs...)
	}
}

// logText 文本格式输出（传统 log.Printf 风格）
func (l *Logger) logText(level LogLevel, module, msg string, kvs ...interface{}) {
	// 构建额外字段字符串
	extra := ""
	for i := 0; i+1 < len(kvs); i += 2 {
		extra += fmt.Sprintf(" %v=%v", kvs[i], kvs[i+1])
	}

	prefix := ""
	switch level {
	case LogLevelWarn:
		prefix = "[WARN]"
	case LogLevelError:
		prefix = "[ERROR]"
	default:
		prefix = "[INFO]"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	log.Printf("%s [%s] %s%s", prefix, module, msg, extra)
}

// logJSON JSON 格式输出
func (l *Logger) logJSON(level LogLevel, module, msg string, kvs ...interface{}) {
	entry := map[string]interface{}{
		"time":   time.Now().UTC().Format(time.RFC3339),
		"level":  level.String(),
		"module": module,
		"msg":    msg,
	}
	// 添加额外键值对
	for i := 0; i+1 < len(kvs); i += 2 {
		key := fmt.Sprintf("%v", kvs[i])
		entry[key] = kvs[i+1]
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	l.writer.Write(data)
	l.writer.Write([]byte("\n"))
}

// Format 返回当前日志格式
func (l *Logger) Format() string {
	return l.format
}

// 全局 Logger 实例
var appLogger *Logger

// InitAppLogger 初始化全局 Logger
func InitAppLogger(format string) {
	appLogger = NewLogger(format, os.Stderr)
}

// GetAppLogger 获取全局 Logger（若未初始化返回默认 text logger）
func GetAppLogger() *Logger {
	if appLogger == nil {
		appLogger = NewLogger("text", os.Stderr)
	}
	return appLogger
}
