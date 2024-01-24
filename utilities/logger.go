package utilities

import (
	"fmt"
	"path"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// InitLogger initialises the logger
func InitLogger(logLevel string) {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Errorf("invalid log level %s, defaulting to INFO log level", logLevel)
		level = log.InfoLevel
	}

	if level == log.DebugLevel {
		log.SetReportCaller(true)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				fileName := path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
				return "", fileName
			},
			TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true,
		})
	}

	log.SetLevel(level)
}

// NewLogger returns the logger client
func NewLogger(fName string) *log.Entry {
	return log.WithFields(log.Fields{
		"fn": fmt.Sprintf("%s()", fName),
	})
}

func NewLoggerWithFields(fName string, fields map[string]interface{}) *log.Entry {
	f := log.Fields(fields)
	f["fn"] = fmt.Sprintf("%s()", fName)
	return log.WithFields(f)
}
