package lastlog

import (
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	mu        sync.Mutex
	memLogger *memLogWriter
)

// implements io.Writer
type memLogWriter struct {
	capacity int
	lines    []string
	file     *os.File
}

// io.Writer.Write
func (w *memLogWriter) Write(p []byte) (n int, err error) {
	line := string(p)

	if len(w.lines) >= w.capacity {
		w.lines = w.lines[1:w.capacity]
	}

	// delete \n
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[0 : len(line)-1]
	}

	w.lines = append(w.lines, line)

	return len(p), nil
}

func BeginLogging(fileName string, capacity int) error {
	mu.Lock()
	defer mu.Unlock()

	if memLogger != nil {
		return errors.New("twice BeginLogging call")
	}

	memLogger = &memLogWriter{
		capacity: capacity,
		lines:    []string{},
	}

	writters := []io.Writer{os.Stderr}

	// file writter
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		memLogger.file = file
		writters = append(writters, memLogger.file)
	} else {
		log.Printf("BeginLogging: Can not open \"%s\" file: %v", fileName, err)
	}

	// mem writter
	writters = append(writters, memLogger)

	log.SetOutput(io.MultiWriter(writters...))

	return nil
}

func LastLines() []string {
	mu.Lock()
	defer mu.Unlock()
	return memLogger.lines
}

func LastText() string {
	return strings.Join(memLogger.lines, "\n")
}

func EndLogging() error {
	mu.Lock()
	defer mu.Unlock()

	if memLogger == nil {
		return errors.New("call EndLogging without BeginLogging")
	}

	log.SetOutput(os.Stderr)

	if memLogger.file != nil {
		err := memLogger.file.Close()
		if err != nil {
			return err
		}
	}

	memLogger = nil

	return nil
}
