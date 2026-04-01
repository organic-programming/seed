package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type runtimeConfig struct {
	ConfigDir    string
	ConfigRelDir string
	RepoRoot     string
	Root         rootConfig
	Paths        repoPaths
	SuiteName    string
	SuitePath    string
	Suite        suiteConfig
}

type rootConfig struct {
	Storage  storageConfig  `mapstructure:"storage"`
	Defaults defaultsConfig `mapstructure:"defaults"`
}

type storageConfig struct {
	Reports   string `mapstructure:"reports"`
	Archives  string `mapstructure:"archives"`
	Artifacts string `mapstructure:"artifacts"`
	TempAlias string `mapstructure:"temp_alias"`
}

type defaultsConfig struct {
	Suite         string            `mapstructure:"suite"`
	Source        string            `mapstructure:"source"`
	Lane          string            `mapstructure:"lane"`
	ArchivePolicy map[string]string `mapstructure:"archive_policy"`
}

type suiteConfig struct {
	Description string                       `mapstructure:"description"`
	Steps       map[string]suiteStepConfig   `mapstructure:"steps"`
	Profiles    map[string]suiteProfileLanes `mapstructure:"profiles"`
}

type suiteStepConfig struct {
	Workdir     string   `mapstructure:"workdir"`
	Prereqs     []string `mapstructure:"prereqs"`
	Command     string   `mapstructure:"command"`
	Script      string   `mapstructure:"script"`
	Args        []string `mapstructure:"args"`
	Description string   `mapstructure:"description"`
}

type suiteProfileLanes struct {
	Regression  []string `mapstructure:"regression"`
	Progression []string `mapstructure:"progression"`
}

func loadRepoConfig(configDir string) (*runtimeConfig, error) {
	absConfigDir, err := resolveConfigDir(configDir)
	if err != nil {
		return nil, err
	}
	rootCfg, err := readRootConfig(absConfigDir)
	if err != nil {
		return nil, err
	}
	repoRoot, err := detectRepoRootFrom(absConfigDir)
	if err != nil {
		return nil, err
	}
	configRelDir, err := filepath.Rel(repoRoot, absConfigDir)
	if err != nil {
		return nil, err
	}
	paths, err := newRepoPaths(repoRoot, absConfigDir, rootCfg)
	if err != nil {
		return nil, err
	}
	return &runtimeConfig{
		ConfigDir:    absConfigDir,
		ConfigRelDir: filepath.ToSlash(configRelDir),
		RepoRoot:     repoRoot,
		Root:         rootCfg,
		Paths:        paths,
	}, nil
}

func loadRunConfig(configDir string, suiteName string) (*runtimeConfig, error) {
	cfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(suiteName)
	if name == "" {
		name = strings.TrimSpace(cfg.Root.Defaults.Suite)
	}
	if name == "" {
		return nil, fmt.Errorf("suite is required")
	}
	suitePath := filepath.Join(cfg.ConfigDir, "suites", name+".yaml")
	suite, err := readSuiteConfig(suitePath)
	if err != nil {
		return nil, err
	}
	cfg.SuiteName = name
	cfg.SuitePath = suitePath
	cfg.Suite = suite
	return cfg, nil
}

func readRootConfig(configDir string) (rootConfig, error) {
	v := viper.New()
	v.SetConfigName("ader")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	if err := v.ReadInConfig(); err != nil {
		return rootConfig{}, err
	}
	var cfg rootConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return rootConfig{}, err
	}
	applyRootDefaults(&cfg)
	return cfg, nil
}

func readSuiteConfig(path string) (suiteConfig, error) {
	if !fileExists(path) {
		return suiteConfig{}, fmt.Errorf("suite file not found: %s", path)
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return suiteConfig{}, err
	}
	var cfg suiteConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return suiteConfig{}, err
	}
	if len(cfg.Steps) == 0 {
		return suiteConfig{}, fmt.Errorf("suite file %s does not define any steps", path)
	}
	if len(cfg.Profiles) == 0 {
		return suiteConfig{}, fmt.Errorf("suite file %s does not define any profiles", path)
	}
	for id, step := range cfg.Steps {
		command := strings.TrimSpace(step.Command)
		script := strings.TrimSpace(step.Script)
		switch {
		case script == "" && len(step.Args) > 0:
			return suiteConfig{}, fmt.Errorf("suite file %s step %q cannot define args without script", path, id)
		case command == "" && script == "":
			return suiteConfig{}, fmt.Errorf("suite file %s step %q must define exactly one of command or script", path, id)
		case command != "" && script != "":
			return suiteConfig{}, fmt.Errorf("suite file %s step %q cannot define both command and script", path, id)
		}
	}
	return cfg, nil
}

func applyRootDefaults(cfg *rootConfig) {
	if strings.TrimSpace(cfg.Storage.Reports) == "" {
		cfg.Storage.Reports = "reports"
	}
	if strings.TrimSpace(cfg.Storage.Archives) == "" {
		cfg.Storage.Archives = "archives"
	}
	if strings.TrimSpace(cfg.Storage.Artifacts) == "" {
		cfg.Storage.Artifacts = ".artifacts"
	}
	if strings.TrimSpace(cfg.Storage.TempAlias) == "" {
		cfg.Storage.TempAlias = ".t"
	}
	if strings.TrimSpace(cfg.Defaults.Source) == "" {
		cfg.Defaults.Source = "committed"
	}
	if strings.TrimSpace(cfg.Defaults.Lane) == "" {
		cfg.Defaults.Lane = "regression"
	}
	if cfg.Defaults.ArchivePolicy == nil {
		cfg.Defaults.ArchivePolicy = map[string]string{}
	}
	for profile, value := range map[string]string{
		"quick":       "never",
		"unit":        "never",
		"integration": "never",
		"full":        "auto",
		"stress":      "never",
	} {
		if strings.TrimSpace(cfg.Defaults.ArchivePolicy[profile]) == "" {
			cfg.Defaults.ArchivePolicy[profile] = value
		}
	}
}

func resolveConfigDir(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("config dir is required")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("config dir %s is not a directory", abs)
	}
	return abs, nil
}

func newRepoPaths(repoRoot string, configDir string, cfg rootConfig) (repoPaths, error) {
	return repoPaths{
		RepoRoot:       repoRoot,
		ConfigDir:      configDir,
		ArtifactsDir:   filepath.Join(configDir, filepath.FromSlash(cfg.Storage.Artifacts)),
		LocalSuiteDir:  filepath.Join(configDir, filepath.FromSlash(cfg.Storage.Artifacts), "local-suite"),
		ToolCacheDir:   filepath.Join(configDir, filepath.FromSlash(cfg.Storage.Artifacts), "tool-cache"),
		ReportsDir:     filepath.Join(configDir, filepath.FromSlash(cfg.Storage.Reports)),
		ArchivesDir:    filepath.Join(configDir, filepath.FromSlash(cfg.Storage.Archives)),
		ShortTempAlias: filepath.Join(configDir, filepath.FromSlash(cfg.Storage.TempAlias)),
	}, nil
}

func detectRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return detectRepoRootFrom(cwd)
}

func detectRepoRootFrom(start string) (string, error) {
	cmd := exec.Command("git", "-C", start, "rev-parse", "--show-toplevel")
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output)), nil
	}
	current := start
	for {
		if dirExists(filepath.Join(current, ".git")) || fileExists(filepath.Join(current, ".git")) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("unable to detect repository root from %s", start)
}
