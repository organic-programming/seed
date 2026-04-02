package engine

import (
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	progressHeartbeatInterval     = 5 * time.Second
	progressHeartbeatPollInterval = 250 * time.Millisecond
)

type progressReporter struct {
	out      io.Writer
	interval time.Duration
	poll     time.Duration
	mu       sync.Mutex
}

func newProgressReporter(out io.Writer) *progressReporter {
	return &progressReporter{
		out:      out,
		interval: progressHeartbeatInterval,
		poll:     progressHeartbeatPollInterval,
	}
}

func (r *progressReporter) enabled() bool {
	return r != nil && r.out != nil
}

func (r *progressReporter) Write(data []byte) (int, error) {
	if !r.enabled() {
		return len(data), nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out.Write(data)
}

func (r *progressReporter) printf(format string, args ...any) {
	if !r.enabled() {
		return
	}
	_, _ = fmt.Fprintf(r, format, args...)
}

func (r *progressReporter) phase(message string) {
	r.printf("[phase] %s\n", message)
}

func (r *progressReporter) withPhase(label string, done string, fn func() error) error {
	if !r.enabled() {
		return fn()
	}
	start := time.Now()
	r.phase(label)
	monitor := r.startMonitor(label, "")
	defer monitor.stop()
	if err := fn(); err != nil {
		return err
	}
	r.phase(fmt.Sprintf("%s (%s)", done, formatProgressDuration(time.Since(start))))
	return nil
}

func (r *progressReporter) startMonitor(label string, suffix string) *heartbeatMonitor {
	monitor := &heartbeatMonitor{
		reporter:      r,
		label:         label,
		suffix:        suffix,
		start:         time.Now(),
		lastOutput:    time.Now(),
		lastHeartbeat: time.Time{},
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	go monitor.loop()
	return monitor
}

type heartbeatMonitor struct {
	reporter      *progressReporter
	label         string
	suffix        string
	start         time.Time
	lastOutput    time.Time
	lastHeartbeat time.Time
	stopCh        chan struct{}
	doneCh        chan struct{}
	mu            sync.Mutex
}

func (m *heartbeatMonitor) touch() {
	m.mu.Lock()
	m.lastOutput = time.Now()
	m.mu.Unlock()
}

func (m *heartbeatMonitor) stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *heartbeatMonitor) writer() io.Writer {
	return &monitorWriter{reporter: m.reporter, monitor: m}
}

func (m *heartbeatMonitor) loop() {
	defer close(m.doneCh)
	ticker := time.NewTicker(m.reporter.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var emit bool
			m.mu.Lock()
			if now.Sub(m.lastOutput) >= m.reporter.interval && (m.lastHeartbeat.IsZero() || now.Sub(m.lastHeartbeat) >= m.reporter.interval) {
				m.lastHeartbeat = now
				emit = true
			}
			m.mu.Unlock()
			if emit {
				elapsed := formatProgressDuration(now.Sub(m.start))
				if m.suffix != "" {
					m.reporter.printf("[wait] %s still running (%s, %s)\n", m.label, elapsed, m.suffix)
				} else {
					m.reporter.printf("[wait] %s still running (%s)\n", m.label, elapsed)
				}
			}
		case <-m.stopCh:
			return
		}
	}
}

type monitorWriter struct {
	reporter *progressReporter
	monitor  *heartbeatMonitor
}

func (w *monitorWriter) Write(data []byte) (int, error) {
	if w.monitor != nil {
		w.monitor.touch()
	}
	if w.reporter == nil {
		return len(data), nil
	}
	return w.reporter.Write(data)
}

func formatProgressDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Round(time.Second).String()
}
