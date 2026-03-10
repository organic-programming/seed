package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// TeeWriter writes each line to both the terminal and a log file,
// prefixing every line with a human-readable timestamp.
type TeeWriter struct {
	Terminal io.Writer
	LogFile  *os.File

	mu sync.Mutex
}

func TimestampNow() string {
	now := time.Now()
	return fmt.Sprintf("%s_%03d", now.Format("2006_01_02_15_04_05"), now.Nanosecond()/int(time.Millisecond))
}

func (tw *TeeWriter) WriteLine(line string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")

	prefixed := TimestampNow()
	if line != "" {
		prefixed += " " + line
	}
	prefixed += "\n"

	if tw.Terminal != nil {
		_, _ = io.WriteString(tw.Terminal, prefixed)
	}

	if tw.LogFile != nil {
		_, _ = io.WriteString(tw.LogFile, prefixed)
	}
}
