package main

import (
	"bytes"
	"fmt"
	"os"
	"time"
)

const (
	logLevelDebug = iota
	logLevelInfo
	logLevelWarning
	logLevelError
	logLevelFatal
)

var (
	logLevel = logLevelInfo
)

func Debugf(format string, args ...interface{}) {
	log(logLevelDebug, fmt.Sprintf(format, args...))
}

func Debug(args ...interface{}) {
	log(logLevelDebug, fmt.Sprint(args...))
}

func Infof(format string, args ...interface{}) {
	log(logLevelInfo, fmt.Sprintf(format, args...))
}

func Info(args ...interface{}) {
	log(logLevelInfo, fmt.Sprint(args...))
}

func Warningf(format string, args ...interface{}) {
	log(logLevelWarning, fmt.Sprintf(format, args...))
}

func Warning(args ...interface{}) {
	log(logLevelWarning, fmt.Sprint(args...))
}

func Errorf(format string, args ...interface{}) {
	log(logLevelError, fmt.Sprintf(format, args...))
}

func Error(args ...interface{}) {
	log(logLevelError, fmt.Sprint(args...))
}

func Fatalf(format string, args ...interface{}) {
	log(logLevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func Fatal(args ...interface{}) {
	log(logLevelFatal, fmt.Sprint(args...))
	os.Exit(1)
}

func log(level int, message string) {
	if level < logLevel || level > logLevelFatal {
		return
	}

	var buf bytes.Buffer
	buf.WriteString(time.Now().Format("2006/01/02 15:04:05 "))
	switch level {
	case logLevelDebug:
		buf.WriteString("[D] ")
	case logLevelInfo:
		buf.WriteString("[I] ")
	case logLevelWarning:
		buf.WriteString("[W] ")
	case logLevelError:
		buf.WriteString("[E] ")
	case logLevelFatal:
		buf.WriteString("[F] ")
	}

	buf.WriteString(message)
	buf.WriteByte('\n')
	_, _ = fmt.Fprintf(os.Stderr, buf.String())
}
