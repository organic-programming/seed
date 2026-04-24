package integration

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/go-holons/pkg/observability"
)

type ObservableOptions struct {
	Families  string
	ListenURI string
}

type ObservableHandle struct {
	address string
	process *ProcessHandle
	runDir  string
	slug    string
	uid     string
}

func (s *Sandbox) SpawnObservable(t *testing.T, slug string, opts ObservableOptions) *ObservableHandle {
	t.Helper()
	families := strings.TrimSpace(opts.Families)
	if families == "" {
		families = "logs,metrics,events"
	}
	listenURI := strings.TrimSpace(opts.ListenURI)
	if listenURI == "" {
		listenURI = "tcp://127.0.0.1:0"
	}
	process := s.StartProcess(t, RunOptions{}, "run", slug, "--observe="+families, "--listen", listenURI)
	address := process.WaitForListenAddress(t, ProcessStartTimeout)
	runRoot := filepath.Join(s.OPPATH, "run")

	var runDir string
	var meta observability.MetaJSON
	WaitUntil(t, 30*time.Second, func() bool {
		matches, _ := filepath.Glob(filepath.Join(runRoot, slug, "*", "meta.json"))
		for _, match := range matches {
			dir := filepath.Dir(match)
			m, err := observability.ReadMetaJSON(dir)
			if err == nil && m.Address == address && m.UID != "" {
				runDir = dir
				meta = m
				return true
			}
		}
		return false
	})

	return &ObservableHandle{
		address: address,
		process: process,
		runDir:  runDir,
		slug:    meta.Slug,
		uid:     meta.UID,
	}
}

func (h *ObservableHandle) Address() string { return h.address }

func (h *ObservableHandle) RunDir() string { return h.runDir }

func (h *ObservableHandle) Slug() string { return h.slug }

func (h *ObservableHandle) UID() string { return h.uid }

func (h *ObservableHandle) Stop(t *testing.T) {
	t.Helper()
	if h != nil && h.process != nil {
		h.process.Stop(t)
	}
}
