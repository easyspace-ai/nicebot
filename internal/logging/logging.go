package logging

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	once   sync.Once
	logger *log.Logger
)

func Logger() *log.Logger {
	once.Do(func() {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	})
	return logger
}

func Configure(level, filePath string) (func(), error) {
	_ = level // level is currently advisory; kept for 1:1 config parity.
	lvl := strings.ToUpper(strings.TrimSpace(level))
	_ = lvl

	if filePath == "" {
		return func() {}, nil
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return func() {}, err
	}

	mw := io.MultiWriter(os.Stdout, f)
	Logger().SetOutput(mw)

	return func() { _ = f.Close() }, nil
}
