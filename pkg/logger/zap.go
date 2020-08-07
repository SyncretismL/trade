package logger

import (
	"fmt"
)

//Fields  ...
type Fields map[string]interface{}

const (
	//Debug has verbose message
	Debug = "debug"
	//Info is default log level
	Info = "info"
	//Warn is for logging messages about possible issues
	Warn = "warn"
	//Error is for logging errors
	Error = "error"
	//Fatal is for logging fatal messages. The system shutdown after logging the message.
	Fatal = "fatal"
)

//Logger is our contract for the logger
type Logger interface {
	Debugf(format string, args ...interface{})
	Debugw(msg string, keysAndValues ...interface{})

	Infof(format string, args ...interface{})
	Infow(msg string, keysAndValues ...interface{})

	Warnf(format string, args ...interface{})
	Warnw(msg string, keysAndValues ...interface{})

	Errorf(format string, args ...interface{})
	Errorw(msg string, keysAndValues ...interface{})

	Fatalf(format string, args ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})

	Panicf(format string, args ...interface{})
	Panicw(msg string, keysAndValues ...interface{})

	WithFields(keyValues Fields) Logger
}

// Configuration stores the config for the logger
// For some loggers there can only be one level across writers, for such the level of Console is picked by default
type Configuration struct {
	EnableConsole     bool
	ConsoleJSONFormat bool
	ConsoleLevel      string
	EnableFile        bool
	FileJSONFormat    bool
	FileLevel         string
	FileLocation      string
}

//NewLogger returns an instance of logger
func NewLogger() (Logger, error) {
	config := Configuration{
		EnableConsole:     true,
		ConsoleLevel:      Debug,
		ConsoleJSONFormat: true,
		EnableFile:        true,
		FileLevel:         Info,
		FileJSONFormat:    false,
		FileLocation:      "./logs.log",
	}

	logger, err := newZapLogger(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create new zap logger: %s", err)
	}

	return logger, nil
}
