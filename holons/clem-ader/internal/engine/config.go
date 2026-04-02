package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type runtimeConfig struct {
	ConfigDir          string
	ConfigRelDir       string
	CatalogueName      string
	RepoRoot           string
	Root               rootConfig
	Paths              repoPaths
	SuiteName          string
	SuitePath          string
	ChecksPath         string
	Suite              suiteConfig
	EffectiveSuiteYAML string
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
	Source string `mapstructure:"source"`
	Lane   string `mapstructure:"lane"`
}

type checksConfig struct {
	Checks map[string]checkConfig `yaml:"checks"`
}

type checkConfig struct {
	Workdir     string   `yaml:"workdir"`
	Prereqs     []string `yaml:"prereqs"`
	Command     string   `yaml:"command"`
	Script      string   `yaml:"script"`
	Args        []string `yaml:"args"`
	Description string   `yaml:"description"`
}

type suiteConfig struct {
	Description string                        `yaml:"description"`
	Defaults    suiteDefaultsConfig           `yaml:"defaults"`
	Steps       map[string]suiteStepConfig    `yaml:"steps"`
	Profiles    map[string]suiteProfileConfig `yaml:"profiles"`
}

type suiteDefaultsConfig struct {
	Profile string `yaml:"profile"`
}

type suiteStepConfig struct {
	Workdir     string   `yaml:"workdir"`
	Prereqs     []string `yaml:"prereqs"`
	Command     string   `yaml:"command"`
	Script      string   `yaml:"script"`
	Args        []string `yaml:"args"`
	Description string   `yaml:"description"`
	Lane        string   `yaml:"lane"`
}

type rawSuiteConfig struct {
	Description string                        `yaml:"description"`
	Defaults    suiteDefaultsConfig           `yaml:"defaults"`
	Steps       map[string]rawSuiteStepConfig `yaml:"steps"`
	Profiles    map[string]suiteProfileConfig `yaml:"profiles"`
}

type rawSuiteStepConfig struct {
	Check       string   `yaml:"check"`
	Workdir     string   `yaml:"workdir"`
	Prereqs     []string `yaml:"prereqs"`
	Command     string   `yaml:"command"`
	Script      string   `yaml:"script"`
	Args        []string `yaml:"args"`
	Description string   `yaml:"description"`
	Lane        string   `yaml:"lane"`
}

type suiteProfileConfig struct {
	Description string   `yaml:"description"`
	Archive     string   `yaml:"archive"`
	Steps       []string `yaml:"steps"`
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
		ConfigDir:     absConfigDir,
		ConfigRelDir:  filepath.ToSlash(configRelDir),
		CatalogueName: filepath.Base(absConfigDir),
		RepoRoot:      repoRoot,
		Root:          rootCfg,
		Paths:         paths,
	}, nil
}

func loadRunConfig(configDir string, suiteName string) (*runtimeConfig, error) {
	cfg, err := loadRepoConfig(configDir)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(suiteName)
	if name == "" {
		return nil, fmt.Errorf("suite is required")
	}
	checksPath := filepath.Join(cfg.ConfigDir, "checks.yaml")
	checks, err := readChecksConfig(checksPath)
	if err != nil {
		return nil, err
	}
	suitePath := filepath.Join(cfg.ConfigDir, "suites", name+".yaml")
	suite, err := readSuiteConfig(suitePath, checks)
	if err != nil {
		return nil, err
	}
	effectiveSuiteYAML, err := marshalSuiteConfig(suite)
	if err != nil {
		return nil, err
	}
	cfg.SuiteName = name
	cfg.SuitePath = suitePath
	cfg.ChecksPath = checksPath
	cfg.Suite = suite
	cfg.EffectiveSuiteYAML = effectiveSuiteYAML
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

func readChecksConfig(path string) (checksConfig, error) {
	if !fileExists(path) {
		return checksConfig{}, fmt.Errorf("check catalog file not found: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return checksConfig{}, err
	}
	var cfg checksConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return checksConfig{}, err
	}
	if cfg.Checks == nil {
		cfg.Checks = map[string]checkConfig{}
	}
	for id, check := range cfg.Checks {
		command := strings.TrimSpace(check.Command)
		script := strings.TrimSpace(check.Script)
		switch {
		case script == "" && len(check.Args) > 0:
			return checksConfig{}, fmt.Errorf("check catalog %s entry %q cannot define args without script", path, id)
		case command == "" && script == "":
			return checksConfig{}, fmt.Errorf("check catalog %s entry %q must define exactly one of command or script", path, id)
		case command != "" && script != "":
			return checksConfig{}, fmt.Errorf("check catalog %s entry %q cannot define both command and script", path, id)
		}
	}
	return cfg, nil
}

func readSuiteConfig(path string, checks checksConfig) (suiteConfig, error) {
	if !fileExists(path) {
		return suiteConfig{}, fmt.Errorf("suite file not found: %s", path)
	}
	if err := validateSuiteYAMLShape(path); err != nil {
		return suiteConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return suiteConfig{}, err
	}
	var raw rawSuiteConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return suiteConfig{}, err
	}
	return materializeSuite(path, raw, checks)
}

func materializeSuite(path string, raw rawSuiteConfig, checks checksConfig) (suiteConfig, error) {
	if len(raw.Steps) == 0 {
		return suiteConfig{}, fmt.Errorf("suite file %s does not define any steps", path)
	}
	if len(raw.Profiles) == 0 {
		return suiteConfig{}, fmt.Errorf("suite file %s does not define any profiles", path)
	}
	out := suiteConfig{
		Description: raw.Description,
		Defaults:    raw.Defaults,
		Steps:       make(map[string]suiteStepConfig, len(raw.Steps)),
		Profiles:    make(map[string]suiteProfileConfig, len(raw.Profiles)),
	}
	for id, step := range raw.Steps {
		resolved, err := materializeSuiteStep(path, id, step, checks)
		if err != nil {
			return suiteConfig{}, err
		}
		out.Steps[id] = resolved
	}
	for profile, entry := range raw.Profiles {
		if archive := normalizeArchivePolicy(entry.Archive); archive != "auto" && archive != "always" && archive != "never" {
			return suiteConfig{}, fmt.Errorf("suite file %s profile %q has invalid archive policy %q", path, profile, entry.Archive)
		}
		for _, stepID := range entry.Steps {
			stepID = strings.TrimSpace(stepID)
			if stepID == "" {
				continue
			}
			if _, ok := out.Steps[stepID]; !ok {
				return suiteConfig{}, fmt.Errorf("suite file %s references unknown step %q in profile %q", path, stepID, profile)
			}
		}
		out.Profiles[profile] = entry
	}
	if profile := normalizeProfile(out.Defaults.Profile); profile != "" {
		if _, ok := out.Profiles[profile]; !ok {
			return suiteConfig{}, fmt.Errorf("suite file %s default profile %q is not defined", path, out.Defaults.Profile)
		}
		out.Defaults.Profile = profile
	}
	return out, nil
}

func materializeSuiteStep(path string, id string, step rawSuiteStepConfig, checks checksConfig) (suiteStepConfig, error) {
	checkID := strings.TrimSpace(step.Check)
	lane := normalizeStepLane(step.Lane)
	if lane != "" && lane != "progression" && lane != "regression" {
		return suiteStepConfig{}, fmt.Errorf("suite file %s step %q has invalid lane %q", path, id, step.Lane)
	}
	if checkID != "" {
		if strings.TrimSpace(step.Workdir) != "" || len(step.Prereqs) > 0 || strings.TrimSpace(step.Command) != "" || strings.TrimSpace(step.Script) != "" || len(step.Args) > 0 || strings.TrimSpace(step.Description) != "" {
			return suiteStepConfig{}, fmt.Errorf("suite file %s step %q cannot combine check with inline execution fields", path, id)
		}
		check, ok := checks.Checks[checkID]
		if !ok {
			return suiteStepConfig{}, fmt.Errorf("suite file %s step %q references unknown check %q", path, id, checkID)
		}
		return suiteStepConfig{
			Workdir:     check.Workdir,
			Prereqs:     append([]string(nil), check.Prereqs...),
			Command:     check.Command,
			Script:      check.Script,
			Args:        append([]string(nil), check.Args...),
			Description: check.Description,
			Lane:        lane,
		}, nil
	}
	command := strings.TrimSpace(step.Command)
	script := strings.TrimSpace(step.Script)
	switch {
	case script == "" && len(step.Args) > 0:
		return suiteStepConfig{}, fmt.Errorf("suite file %s step %q cannot define args without script", path, id)
	case command == "" && script == "":
		return suiteStepConfig{}, fmt.Errorf("suite file %s step %q must define either check or exactly one of command or script", path, id)
	case command != "" && script != "":
		return suiteStepConfig{}, fmt.Errorf("suite file %s step %q cannot define both command and script", path, id)
	}
	return suiteStepConfig{
		Workdir:     step.Workdir,
		Prereqs:     append([]string(nil), step.Prereqs...),
		Command:     step.Command,
		Script:      step.Script,
		Args:        append([]string(nil), step.Args...),
		Description: step.Description,
		Lane:        lane,
	}, nil
}

func validateSuiteYAMLShape(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("suite file %s root must be a mapping", path)
	}
	root := doc.Content[0]
	for _, field := range []string{"generation", "generated_lanes", "generated_groups"} {
		if _, ok := mappingValue(root, field); ok {
			return fmt.Errorf("suite file %s uses obsolete syntax %q", path, field)
		}
	}
	profilesNode, ok := mappingValue(root, "profiles")
	if !ok || profilesNode.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(profilesNode.Content); index += 2 {
		profileName := strings.TrimSpace(profilesNode.Content[index].Value)
		profileNode := profilesNode.Content[index+1]
		if profileNode.Kind != yaml.MappingNode {
			return fmt.Errorf("suite file %s profile %q must be a mapping", path, profileName)
		}
		if _, ok := mappingValue(profileNode, "regression"); ok {
			return fmt.Errorf("suite file %s profile %q uses the old regression/progression format; migrate to explicit step lanes", path, profileName)
		}
		if _, ok := mappingValue(profileNode, "progression"); ok {
			return fmt.Errorf("suite file %s profile %q uses the old regression/progression format; migrate to explicit step lanes", path, profileName)
		}
		if _, ok := mappingValue(profileNode, "generated_groups"); ok {
			return fmt.Errorf("suite file %s uses obsolete syntax %q", path, "generated_groups")
		}
		if stepsNode, ok := mappingValue(profileNode, "steps"); ok && stepsNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("suite file %s profile %q field %q must be a sequence", path, profileName, "steps")
		}
	}
	return nil
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
}

func marshalSuiteConfig(cfg suiteConfig) (string, error) {
	type snapshotSuite struct {
		Description string                        `yaml:"description,omitempty"`
		Defaults    suiteDefaultsConfig           `yaml:"defaults,omitempty"`
		Steps       map[string]suiteStepConfig    `yaml:"steps"`
		Profiles    map[string]suiteProfileConfig `yaml:"profiles"`
	}
	out, err := yaml.Marshal(snapshotSuite{
		Description: cfg.Description,
		Defaults:    cfg.Defaults,
		Steps:       cfg.Steps,
		Profiles:    cfg.Profiles,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
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

func orderedCheckIDs(checks map[string]checkConfig) []string {
	out := make([]string, 0, len(checks))
	for id := range checks {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
