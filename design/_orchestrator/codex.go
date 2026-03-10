package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config holds the run configuration for the orchestrator
type Config struct {
	Sets       []string
	Model      string
	RootFolder string
	StateFile  string
}

// State tracks which tasks have been completed
type State struct {
	CompletedTasks map[string]bool `json:"completed_tasks"`
}

// ArrayFlags represents a slice of string arguments for flags
type ArrayFlags []string

func (i *ArrayFlags) String() string {
	return strings.Join(*i, ", ")
}

func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var setsFlag ArrayFlags
	var model string
	var rootFolder string

	flag.Var(&setsFlag, "set", "Set to orchestrate (can be specified multiple times, e.g., --set v0.4 --set v0.5)")
	flag.StringVar(&model, "model", "gpt-5.4-thinking", "Model to use for codex (e.g., gpt-5.4-thinking, claude-3.5-sonnet, gemini-3.1-flash)")
	flag.StringVar(&rootFolder, "root", ".", "Root folder containing git modules and the design folder")
	flag.Parse()

	if len(setsFlag) == 0 {
		log.Fatalf("Please provide at least one set via --set (e.g., --set v0.4 --set v0.5)\n")
	}

	config := Config{
		Sets:  setsFlag,
		Model: model,

		RootFolder: rootFolder,
		StateFile:  filepath.Join(rootFolder, ".codex_orchestrator_state.json"),
	}

	log.Printf("Starting Codex Orchestrator with sets: %v. Model: %s\n", config.Sets, config.Model)

	// Step 1: Load State
	state := loadState(config.StateFile)

	// Step 2: Iterate through sets independently
	for i, set := range config.Sets {
		log.Printf("\n--- Processing Set: %s ---\n", set)

		// Discover the target directory inside design/ dynamically
		targetDir, parentFolder, err := findSetDir(config.RootFolder, set)
		if err != nil {
			log.Fatalf("Error finding set directory: %v", err)
		}

		targetBranch := fmt.Sprintf("%s-%s-dev", parentFolder, set)

		// Determine base branch
		var baseBranch string
		if i > 0 {
			// Infer base branch from previous set dynamically
			_, prevParent, _ := findSetDir(config.RootFolder, config.Sets[i-1])
			baseBranch = fmt.Sprintf("%s-%s-dev", prevParent, config.Sets[i-1])
		} else {
			baseBranch = getCurrentBranch(config.RootFolder)
			if !strings.HasSuffix(baseBranch, "dev") {
				log.Fatalf("Error: Current branch '%s' does not end with 'dev'. Exiting.", baseBranch)
			}
		}

		// Ensure Branch Consistency Sandbox
		err = ensureGitConsistency(config.RootFolder, baseBranch, targetBranch)
		if err != nil {
			log.Fatalf("Git consistency check failed: %v", err)
		}

		// Configure MCP
		ensureMCP(config.RootFolder)

		// Step 3: Run Tasks for Set
		runTasksInDir(config, state, targetDir)
	}

	log.Println("\nAll sets processed successfully.")
}

// findSetDir looks for the set folder inside design/* and returns its absolute path and its parent folder name
func findSetDir(root, set string) (string, string, error) {
	designDir := filepath.Join(root, "design")
	dirs, err := os.ReadDir(designDir)
	if err != nil {
		return "", "", err
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		// Look for the set folder inside e.g. "design/grace-op"
		possible := filepath.Join(designDir, d.Name(), set)
		if info, err := os.Stat(possible); err == nil && info.IsDir() {
			return possible, d.Name(), nil
		}
	}
	return "", "", fmt.Errorf("set '%s' not found in any design/* subfolder", set)
}

// ensureMCP adds an MCP capability to codex if it is not already present
func ensureMCP(root string) {
	log.Printf("Verifying Codex MCP capabilities for workflows...")
	cmd := exec.Command("codex", "mcp", "list")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "workflows") {
		log.Println("MCP 'workflows' is already configured.")
		return
	}

	log.Println("MCP 'workflows' not found. Installing via codex mcp add...")
	// codex mcp add workflows -- npx -y @modelcontextprotocol/server-filesystem .agent/workflows
	addCmd := exec.Command("codex", "mcp", "add", "workflows", "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", ".agent/workflows")
	addCmd.Dir = root
	if addOut, addErr := addCmd.CombinedOutput(); addErr != nil {
		log.Printf("Failed to set up MCP workflows: %s. Tasks will proceed without it.", string(addOut))
	} else {
		log.Println("MCP 'workflows' successfully configured.")
	}
}

// loadState reads the JSON state file tracking completed tasks
func loadState(path string) *State {
	state := &State{
		CompletedTasks: make(map[string]bool),
	}
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, state)
	}
	return state
}

// saveState writes the state to disk
func saveState(path string, state *State) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("Failed to save state: %v\n", err)
		return
	}
	os.WriteFile(path, data, 0644)
}

// getCurrentBranch gets the branch of a repository or subdirectory
func getCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ensureGitConsistency checks the base branch and derives the target feature branch across submodules
func ensureGitConsistency(root, base, target string) error {
	log.Printf("Checking Git consistency... Base: %s, Target: %s", base, target)

	// Check Main Repo
	currentMain := getCurrentBranch(root)

	if currentMain == target {
		log.Printf("Main repo is already on target branch '%s'. Verifying submodules...", target)
		return verifySubmodulesBranch(root, target)
	}

	if currentMain != base {
		return fmt.Errorf("main repo is on '%s', expected base branch '%s'", currentMain, base)
	}

	// Verify all submodules are on base branch
	err := verifySubmodulesBranch(root, base)
	if err != nil {
		return fmt.Errorf("submodule verification failed: %w", err)
	}

	log.Printf("All repositories are consistently on base branch '%s'. Deriving target branch '%s'...", base, target)

	// Branch Main Repo
	cmd := exec.Command("git", "checkout", "-b", target)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to branch main repo: %s", string(out))
	}

	// Branch Submodules
	subCmd := exec.Command("git", "submodule", "foreach", "--recursive", fmt.Sprintf("git checkout -b %s", target))
	subCmd.Dir = root
	if out, err := subCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to branch submodules: %s", string(out))
	}

	log.Printf("Successfully derived and checked out '%s' across all modules.", target)
	return nil
}

func verifySubmodulesBranch(root, expectedBranch string) error {
	// Use git submodule foreach to print the current branch of each submodule
	// Format: git rev-parse --abbrev-ref HEAD
	cmd := exec.Command("git", "submodule", "foreach", "--recursive", "git rev-parse --abbrev-ref HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run submodule foreach: %w", err)
	}

	output := string(out)
	lines := strings.Split(output, "\n")

	var currentSubmodule string
	unmatched := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// git submodule foreach prints "Entering 'submodule_path'" before the command output
		if strings.HasPrefix(line, "Entering '") {
			currentSubmodule = strings.TrimPrefix(line, "Entering '")
			currentSubmodule = strings.TrimSuffix(currentSubmodule, "'")
			continue
		}

		// The line content should be the branch name
		if line != expectedBranch {
			unmatched = append(unmatched, fmt.Sprintf("%s (on '%s', expected '%s')", currentSubmodule, line, expectedBranch))
		}
	}

	if len(unmatched) > 0 {
		return fmt.Errorf("the following submodules are not on the expected branch:%s", "\n  - "+strings.Join(unmatched, "\n  - "))
	}

	return nil
}

// runTasksInDir finds and executes all markdown tasks for a specific set directory
func runTasksInDir(cfg Config, state *State, targetDir string) {
	files, err := filepath.Glob(filepath.Join(targetDir, "*.md"))
	if err != nil || len(files) == 0 {
		log.Printf("No markdown tasks found in %s\n", targetDir)
		return
	}

	for _, file := range files {
		// Example filter: if file is not an OP_TASK or DESIGN task, skip.
		// For now we process all .md files as tasks.

		if state.CompletedTasks[file] {
			log.Printf("Skipping completed task: %s\n", filepath.Base(file))
			continue
		}

		log.Printf("Executing task: %s\n", filepath.Base(file))
		err := executeTask(cfg, file)
		if err != nil {
			log.Fatalf("Failed to execute task '%s': %v\n", file, err)
		}

		// Mark as complete and save
		state.CompletedTasks[file] = true
		saveState(cfg.StateFile, state)
	}
}

// executeTask runs the task via codex LLM orchestrator
func executeTask(cfg Config, taskFile string) error {
	logFile := taskFile + ".log"

	// Open log file
	out, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("could not create log file: %w", err)
	}
	defer out.Close()

	// Example codex command: codex exec --model <MODEL> --full-auto "<task prompt here or pass file>"
	// Or pass the file content directly. Let's pass the instruction from file content
	content, err := os.ReadFile(taskFile)
	if err != nil {
		return err
	}

	prompt := string(content) + fmt.Sprintf("\nYou have access to workflows via MCP at %s", filepath.Join(cfg.RootFolder, ".agent", "workflows"))

	// Build codex command
	cmd := exec.Command("codex", "exec", "--model", cfg.Model, "--full-auto", prompt)
	cmd.Dir = cfg.RootFolder

	// Pipe output to both stdout and log file
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read output in a goroutine and write to log & terminal
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			fmt.Fprintln(out, line)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			fmt.Fprintln(out, line)
		}
	}()

	return cmd.Wait()
}
