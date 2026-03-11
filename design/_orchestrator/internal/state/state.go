package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type Outcome string

const (
	OutcomePending  Outcome = "pending"
	OutcomeSuccess  Outcome = "success"
	OutcomeFailed   Outcome = "failed"
	OutcomeDeferred Outcome = "deferred"
)

type TokenUsage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

func (t *TokenUsage) Add(other TokenUsage) {
	t.InputTokens += other.InputTokens
	t.CachedInputTokens += other.CachedInputTokens
	t.OutputTokens += other.OutputTokens
}

type TaskState struct {
	Completed     bool      `json:"completed"`
	Outcome       Outcome   `json:"outcome"`
	ThreadID      string    `json:"thread_id,omitempty"`
	Tokens        TokenUsage `json:"tokens"`
	Phase         string    `json:"phase,omitempty"`
	Attempts      int       `json:"attempts,omitempty"`
	PendingPrompt string    `json:"pending_prompt,omitempty"`
}

type State struct {
	mu             sync.Mutex `json:"-"`
	path           string     `json:"-"`
	Tasks          map[string]TaskState `json:"tasks"`
	CompletedOrder []string   `json:"completed_order"`
}

func Load(stateFile string) *State {
	path := normalizePath(stateFile)
	st := &State{
		path:  path,
		Tasks: make(map[string]TaskState),
	}

	if path == "" {
		return st
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}

	var decoded State
	if err := json.Unmarshal(data, &decoded); err != nil {
		badPath := path + ".bad"
		if renameErr := os.Rename(path, badPath); renameErr != nil {
			fmt.Fprintf(os.Stderr, "warning: state file %s is corrupt (%v); failed to rename to %s: %v\n", path, err, badPath, renameErr)
		} else {
			fmt.Fprintf(os.Stderr, "warning: state file %s is corrupt (%v); moved to %s\n", path, err, badPath)
		}
		return st
	}

	st.Tasks = make(map[string]TaskState, len(decoded.Tasks))
	for taskFile, taskState := range decoded.Tasks {
		st.Tasks[normalizePath(taskFile)] = normalizeTaskState(taskState)
	}
	for _, taskFile := range decoded.CompletedOrder {
		taskFile = normalizePath(taskFile)
		taskState, ok := st.Tasks[taskFile]
		if !ok || !taskState.Completed {
			continue
		}
		if !slices.Contains(st.CompletedOrder, taskFile) {
			st.CompletedOrder = append(st.CompletedOrder, taskFile)
		}
	}

	return st
}

func (s *State) Save() error {
	if s == nil || s.path == "" {
		return nil
	}

	s.mu.Lock()
	payload := struct {
		Tasks          map[string]TaskState `json:"tasks"`
		CompletedOrder []string             `json:"completed_order"`
	}{
		Tasks:          make(map[string]TaskState, len(s.Tasks)),
		CompletedOrder: append([]string(nil), s.CompletedOrder...),
	}
	for taskFile, taskState := range s.Tasks {
		payload.Tasks[taskFile] = normalizeTaskState(taskState)
	}
	path := s.path
	s.mu.Unlock()

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".state-*.json")
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

func (s *State) Path() string {
	if s == nil {
		return ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.path
}

func (s *State) IsCompleted(taskFile string) bool {
	taskState := s.Task(taskFile)
	return taskState.Completed
}

func (s *State) Task(taskFile string) TaskState {
	if s == nil {
		return normalizeTaskState(TaskState{})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return normalizeTaskState(s.Tasks[normalizePath(taskFile)])
}

func (s *State) UpdateTask(taskFile string, update func(*TaskState)) TaskState {
	if s == nil {
		return normalizeTaskState(TaskState{})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizePath(taskFile)
	taskState := normalizeTaskState(s.Tasks[key])
	update(&taskState)
	taskState = normalizeTaskState(taskState)
	s.Tasks[key] = taskState
	s.syncCompletedOrderLocked(key, taskState)

	return taskState
}

func (s *State) SetTask(taskFile string, taskState TaskState) TaskState {
	return s.UpdateTask(taskFile, func(current *TaskState) {
		*current = taskState
	})
}

func (s *State) CompletedResults(setDir string) []string {
	if s == nil {
		return nil
	}

	prefix := normalizeDirPrefix(setDir)

	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]string, 0, len(s.CompletedOrder))
	for _, taskFile := range s.CompletedOrder {
		taskState, ok := s.Tasks[taskFile]
		if !ok || !taskState.Completed {
			continue
		}
		if prefix != "" && !strings.HasPrefix(taskFile, prefix) {
			continue
		}
		results = append(results, taskFile+".result.md")
	}

	return results
}

func (s *State) RemoveSet(setDir string) {
	if s == nil {
		return
	}

	prefix := normalizeDirPrefix(setDir)

	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.CompletedOrder[:0]
	for _, taskFile := range s.CompletedOrder {
		if prefix != "" && strings.HasPrefix(taskFile, prefix) {
			continue
		}
		filtered = append(filtered, taskFile)
	}
	s.CompletedOrder = filtered

	for taskFile := range s.Tasks {
		if prefix != "" && strings.HasPrefix(taskFile, prefix) {
			delete(s.Tasks, taskFile)
		}
	}
}

func (s *State) TasksSnapshot() map[string]TaskState {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]TaskState, len(s.Tasks))
	for taskFile, taskState := range s.Tasks {
		result[taskFile] = taskState
	}

	return result
}

func (s *State) syncCompletedOrderLocked(taskFile string, taskState TaskState) {
	index := slices.Index(s.CompletedOrder, taskFile)
	if taskState.Completed {
		if index == -1 {
			s.CompletedOrder = append(s.CompletedOrder, taskFile)
		}
		return
	}
	if index != -1 {
		s.CompletedOrder = append(s.CompletedOrder[:index], s.CompletedOrder[index+1:]...)
	}
}

func normalizeTaskState(taskState TaskState) TaskState {
	switch taskState.Outcome {
	case OutcomeSuccess:
		taskState.Completed = true
	case OutcomeFailed, OutcomeDeferred, OutcomePending:
		taskState.Completed = false
	default:
		if taskState.Completed {
			taskState.Outcome = OutcomeSuccess
		} else {
			taskState.Outcome = OutcomePending
		}
	}
	return taskState
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}

	return filepath.Clean(path)
}

func normalizeDirPrefix(path string) string {
	path = normalizePath(path)
	if path == "" {
		return ""
	}
	return path + string(os.PathSeparator)
}
