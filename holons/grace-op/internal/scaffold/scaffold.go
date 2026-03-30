package scaffold

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/organic-programming/grace-op/internal/manifestproto"
	templatesfs "github.com/organic-programming/grace-op/templates"
	"gopkg.in/yaml.v3"
)

const catalogRoot = "catalog"

type Param struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

type Metadata struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description,omitempty"`
	Lang        string  `yaml:"lang,omitempty"`
	Params      []Param `yaml:"params,omitempty"`
}

type Entry struct {
	Name        string
	Description string
	Lang        string
	Params      []Param
	dir         string
	alias       string
}

type GenerateOptions struct {
	Dir       string
	Overrides map[string]string
}

type GenerateResult struct {
	Template string `json:"template"`
	Dir      string `json:"dir"`
}

type compositeSpec struct {
	lang   string
	runner string
}

var compositeDaemons = map[string]compositeSpec{
	"go":     {lang: "go", runner: "go-module"},
	"rust":   {lang: "rust", runner: "cargo"},
	"python": {lang: "python", runner: "python"},
	"swift":  {lang: "swift", runner: "swift-package"},
	"kotlin": {lang: "kotlin", runner: "gradle"},
	"dart":   {lang: "dart", runner: "dart"},
	"csharp": {lang: "csharp", runner: "dotnet"},
	"node":   {lang: "node", runner: "npm"},
	"cpp":    {lang: "cpp", runner: "cmake"},
}

var compositeHostUIs = map[string]compositeSpec{
	"swiftui": {lang: "swift", runner: "swift-package"},
	"flutter": {lang: "dart", runner: "flutter"},
	"kotlin":  {lang: "kotlin", runner: "gradle"},
	"web":     {lang: "web", runner: "recipe"},
	"dotnet":  {lang: "csharp", runner: "dotnet"},
	"qt":      {lang: "cpp", runner: "qt-cmake"},
}

func List() ([]Entry, error) {
	dirs, err := fs.ReadDir(templatesfs.FS, catalogRoot)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(dirs))
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		if dir.Name() == "composite-generic" {
			continue
		}
		entry, err := loadEntry(dir.Name())
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	for daemon := range compositeDaemons {
		for hostui := range compositeHostUIs {
			name := fmt.Sprintf("composite-%s-%s", daemon, hostui)
			entries = append(entries, Entry{
				Name:        name,
				Description: fmt.Sprintf("Composite %s daemon + %s host UI assembly", daemon, hostui),
				Lang:        compositeDaemons[daemon].lang + "+" + compositeHostUIs[hostui].lang,
				dir:         "composite-generic",
				alias:       name,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func Generate(templateName, slug string, opts GenerateOptions) (GenerateResult, error) {
	entry, err := resolveEntry(templateName)
	if err != nil {
		return GenerateResult{}, err
	}
	if strings.TrimSpace(slug) == "" {
		return GenerateResult{}, fmt.Errorf("template generation requires <holon-name>")
	}
	baseDir := strings.TrimSpace(opts.Dir)
	if baseDir == "" {
		baseDir = "."
	}
	baseDir, err = filepath.Abs(baseDir)
	if err != nil {
		return GenerateResult{}, err
	}
	outputDir := filepath.Join(baseDir, slug)
	if _, err := os.Stat(outputDir); err == nil {
		return GenerateResult{}, fmt.Errorf("%s already exists", outputDir)
	} else if !os.IsNotExist(err) {
		return GenerateResult{}, err
	}
	ctx, err := buildContext(entry, slug, opts.Overrides)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return GenerateResult{}, err
	}
	if err := fs.WalkDir(templatesfs.FS, filepath.Join(catalogRoot, entry.dir), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == filepath.Join(catalogRoot, entry.dir) || d.Name() == "template.yaml" {
			return nil
		}
		rel, err := filepath.Rel(filepath.Join(catalogRoot, entry.dir), path)
		if err != nil {
			return err
		}
		rel, err = renderPath(rel, ctx)
		if err != nil {
			return err
		}
		dest := filepath.Join(outputDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := fs.ReadFile(templatesfs.FS, path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tmpl") {
			rendered, renderErr := renderContent(string(data), ctx)
			if renderErr != nil {
				return renderErr
			}
			if manifestproto.IsLegacyTemplatePath(rel) {
				dest = filepath.Join(outputDir, filepath.Dir(rel), manifestproto.OutputFileName())
				protoData, protoErr := manifestproto.RenderFromYAML([]byte(rendered), manifestproto.RenderOptions{
					FallbackName: slug,
				})
				if protoErr != nil {
					return protoErr
				}
				return os.WriteFile(dest, protoData, 0o644)
			}
			dest = strings.TrimSuffix(dest, ".tmpl")
			return os.WriteFile(dest, []byte(rendered), 0o644)
		}
		return os.WriteFile(dest, data, 0o644)
	}); err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{Template: entry.Name, Dir: outputDir}, nil
}

func resolveEntry(name string) (Entry, error) {
	if entry, err := loadEntry(name); err == nil {
		return entry, nil
	}
	parts := strings.Split(strings.TrimPrefix(name, "composite-"), "-")
	if len(parts) == 2 && strings.HasPrefix(name, "composite-") {
		if _, ok := compositeDaemons[parts[0]]; ok {
			if _, ok := compositeHostUIs[parts[1]]; ok {
				return Entry{
					Name:        name,
					Description: fmt.Sprintf("Composite %s daemon + %s host UI assembly", parts[0], parts[1]),
					Lang:        compositeDaemons[parts[0]].lang + "+" + compositeHostUIs[parts[1]].lang,
					dir:         "composite-generic",
					alias:       name,
				}, nil
			}
		}
	}
	return Entry{}, fmt.Errorf("unknown template %q", name)
}

func loadEntry(name string) (Entry, error) {
	data, err := fs.ReadFile(templatesfs.FS, filepath.Join(catalogRoot, name, "template.yaml"))
	if err != nil {
		return Entry{}, err
	}
	var meta Metadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return Entry{}, fmt.Errorf("parse template metadata %q: %w", name, err)
	}
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = name
	}
	return Entry{
		Name:        meta.Name,
		Description: meta.Description,
		Lang:        meta.Lang,
		Params:      meta.Params,
		dir:         name,
	}, nil
}

func buildContext(entry Entry, slug string, overrides map[string]string) (map[string]string, error) {
	given, family := splitSlug(slug)
	ctx := map[string]string{
		"UUID":        newUUID(),
		"Slug":        slug,
		"GivenName":   given,
		"FamilyName":  family,
		"GivenTitle":  titleCase(given),
		"FamilyTitle": titleCase(family),
		"Module":      slug,
		"Date":        time.Now().Format("2006-01-02"),
		"PascalSlug":  pascalCase(slug),
		"Template":    entry.Name,
		"Lang":        entry.Lang,
	}
	for _, param := range entry.Params {
		value := strings.TrimSpace(overrides[param.Name])
		if value == "" && strings.TrimSpace(param.Default) != "" {
			rendered, err := renderContent(param.Default, ctx)
			if err != nil {
				return nil, err
			}
			value = strings.TrimSpace(rendered)
		}
		if value == "" && param.Required {
			return nil, fmt.Errorf("template %q requires --set %s=...", entry.Name, param.Name)
		}
		if value != "" {
			ctx[param.Name] = value
			ctx[pascalCase(param.Name)] = value
		}
	}
	if entry.alias != "" && strings.HasPrefix(entry.alias, "composite-") {
		parts := strings.Split(strings.TrimPrefix(entry.alias, "composite-"), "-")
		ctx["DaemonKind"] = parts[0]
		ctx["HostUIKind"] = parts[1]
		ctx["DaemonRunner"] = compositeDaemons[parts[0]].runner
		ctx["HostUIRunner"] = compositeHostUIs[parts[1]].runner
	}
	if _, ok := ctx["service"]; !ok {
		ctx["service"] = pascalCase(family) + "Service"
	}
	if _, ok := ctx["Service"]; !ok {
		ctx["Service"] = ctx["service"]
	}
	return ctx, nil
}

func renderPath(value string, ctx map[string]string) (string, error) {
	return renderContent(value, ctx)
}

func renderContent(value string, ctx map[string]string) (string, error) {
	tmpl, err := template.New("content").Funcs(template.FuncMap{
		"pascal": pascalCase,
		"title":  titleCase,
	}).Parse(value)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func splitSlug(slug string) (string, string) {
	parts := strings.Split(strings.Trim(slug, "-"), "-")
	if len(parts) == 0 {
		return slug, slug
	}
	if len(parts) == 1 {
		return parts[0], parts[0]
	}
	return parts[0], strings.Join(parts[1:], "-")
}

func titleCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func pascalCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, "")
}

func newUUID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		data[0:4],
		data[4:6],
		data[6:8],
		data[8:10],
		data[10:16],
	)
}
