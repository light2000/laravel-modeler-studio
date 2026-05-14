package logger

import (
	"fmt"
	"io"
	"log"
	"os"
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	ErrorLevel
)

var (
	level  = InfoLevel
	logger *log.Logger
)

func Init(logFile string, l Level) error {
	level = l
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("open log file %q: %w", logFile, err)
		}
		writers = append(writers, file)
	}

	multi := io.MultiWriter(writers...)
	logger = log.New(multi, "", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

func Debugf(format string, v ...any) {
	if level <= DebugLevel {
		logger.Output(2, fmt.Sprintf("[DEBUG] "+format, v...))
	}
}

func Infof(format string, v ...any) {
	if level <= InfoLevel {
		logger.Output(2, fmt.Sprintf("[INFO] "+format, v...))
	}
}

func Errorf(format string, v ...any) {
	if level <= ErrorLevel {
		logger.Output(2, fmt.Sprintf("[ERROR] "+format, v...))
	}
}

func Fatalf(format string, args ...any) {
	Errorf(format, args...)
	os.Exit(1)
}
