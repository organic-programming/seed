package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/identity"
	"google.golang.org/grpc"
)

const defaultProbeTimeout = 5 * time.Second

// ProbeFunc is called when a .holon package directory has no .holon.json.
// It receives the absolute package dir path and should return a HolonEntry
// if the holon can be probed (e.g. via stdio Describe). It may also write
// .holon.json as a side effect using WritePackageJSON.
type ProbeFunc func(packageDir string) (*HolonEntry, error)

var (
	probeMu sync.RWMutex
	probeFn ProbeFunc
)

// SetProbe registers an optional fallback probe for packages missing .holon.json.
// Callers can override the SDK-native probe when needed.
func SetProbe(fn ProbeFunc) {
	probeMu.Lock()
	defer probeMu.Unlock()
	probeFn = fn
}

func getProbe() ProbeFunc {
	probeMu.RLock()
	defer probeMu.RUnlock()
	return probeFn
}

// WritePackageJSON writes a .holon.json cache file inside the given package directory.
// The entry's fields are projected into the holon-package/v1 schema.
func WritePackageJSON(packageDir string, entry HolonEntry) error {
	payload := holonPackageJSON{
		Schema: "holon-package/v1",
		Slug:   entry.Slug,
		UUID:   entry.UUID,
		Identity: holonIdentityJSON{
			GivenName:  entry.Identity.GivenName,
			FamilyName: entry.Identity.FamilyName,
			Motto:      entry.Identity.Motto,
			Aliases:    append([]string(nil), entry.Identity.Aliases...),
		},
		Lang:          entry.Identity.Lang,
		Status:        entry.Identity.Status,
		Transport:     entry.Transport,
		Entrypoint:    entry.Entrypoint,
		Architectures: entry.Architectures,
		HasDist:       entry.HasDist,
		HasSource:     entry.HasSource,
	}

	if entry.Manifest != nil {
		payload.Runner = entry.Manifest.Build.Runner
		payload.Kind = entry.Manifest.Kind
	}
	if payload.Runner == "" {
		payload.Runner = entry.Runner
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	target := filepath.Join(packageDir, ".holon.json")
	return os.WriteFile(target, append(data, '\n'), 0o644)
}

// probePackageEntry attempts to resolve a .holon package dir that has no
// .holon.json by calling the registered probe function and then the SDK-native
// stdio Describe fallback.
func probePackageEntry(root, dir, origin string) (HolonEntry, error) {
	if probe := getProbe(); probe != nil {
		entry, err := probe(dir)
		if err == nil && entry != nil {
			return normalizeProbedEntry(root, dir, origin, *entry), nil
		}
		if err != nil && !errorsIsNotExist(err) {
			return HolonEntry{}, err
		}
	}

	return nativeProbePackageEntry(root, dir, origin)
}

func nativeProbePackageEntry(root, dir, origin string) (HolonEntry, error) {
	binaryPath, err := packageBinaryPath(dir)
	if err != nil {
		return HolonEntry{}, err
	}
	entry, err := probeBinaryPath(binaryPath)
	if err != nil {
		return HolonEntry{}, err
	}
	entry.Dir = dir
	entry.PackageRoot = dir
	entry.SourceKind = "package"
	entry.Origin = origin
	entry.RelativePath = relativePath(root, dir)
	entry.HasSource = false
	return normalizeProbedEntry(root, dir, origin, entry), nil
}

func normalizeProbedEntry(root, dir, origin string, entry HolonEntry) HolonEntry {
	absRoot, _ := filepath.Abs(strings.TrimSpace(root))
	absDir, _ := filepath.Abs(dir)
	entry.Dir = absDir
	entry.RelativePath = relativePath(absRoot, absDir)
	entry.Origin = origin
	if entry.SourceKind == "" {
		entry.SourceKind = "package"
	}
	if entry.SourceKind == "package" {
		entry.PackageRoot = absDir
	}
	return entry
}

func probeBinaryPath(binaryPath string) (HolonEntry, error) {
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return HolonEntry{}, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return HolonEntry{}, err
	}
	if info.IsDir() {
		return HolonEntry{}, fmt.Errorf("%s is a directory", absPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()

	holonInfo, err := describeBinaryTarget(ctx, absPath)
	if err != nil {
		return HolonEntry{}, err
	}

	entry := holonEntryFromInfo(*holonInfo, filepath.Dir(absPath), "binary")
	entry.Entrypoint = absPath
	entry.Runner = holonInfo.Runner
	if entry.Manifest == nil {
		entry.Manifest = &Manifest{}
	}
	entry.Manifest.Artifacts.Binary = absPath
	entry.Manifest.Build.Runner = holonInfo.Runner
	entry.Manifest.Build.Main = holonInfo.BuildMain
	return entry, nil
}

func isDirectTransportExpression(expression string) bool {
	switch transportScheme(expression) {
	case "tcp", "unix", "ws", "wss", "http", "https":
		return true
	default:
		return false
	}
}

func transportScheme(expression string) string {
	parsed, err := url.Parse(strings.TrimSpace(expression))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Scheme)
}

func discoverTransportRef(ctx context.Context, target string) HolonRef {
	info, err := describeTransportTarget(ctx, target)
	if err != nil {
		return HolonRef{URL: target, Error: err.Error()}
	}
	return HolonRef{URL: target, Info: info}
}

func describeTransportTarget(ctx context.Context, target string) (*HolonInfo, error) {
	conn, err := grpcclient.Dial(ctx, normalizeDialTarget(target))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return describeConn(ctx, conn)
}

func describeBinaryTarget(ctx context.Context, binaryPath string) (*HolonInfo, error) {
	conn, cmd, err := grpcclient.DialStdio(ctx, binaryPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close()
		terminateCommand(cmd)
	}()

	return describeConn(ctx, conn)
}

func describeConn(ctx context.Context, conn *grpc.ClientConn) (*HolonInfo, error) {
	response, err := holonsv1.NewHolonMetaClient(conn).Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		return nil, err
	}
	return holonInfoFromDescribeResponse(response)
}

func holonInfoFromDescribeResponse(response *holonsv1.DescribeResponse) (*HolonInfo, error) {
	if response == nil || response.GetManifest() == nil {
		return nil, fmt.Errorf("Describe returned no manifest")
	}

	manifest := response.GetManifest()
	identity := manifest.GetIdentity()
	if identity == nil {
		return nil, fmt.Errorf("Describe returned no manifest identity")
	}

	slug := strings.ToLower(strings.Trim(strings.ReplaceAll(identity.GetGivenName()+"-"+strings.TrimSuffix(identity.GetFamilyName(), "?"), " ", "-"), "-"))
	build := manifest.GetBuild()
	artifacts := manifest.GetArtifacts()

	return &HolonInfo{
		Slug: slug,
		UUID: identity.GetUuid(),
		Identity: IdentityInfo{
			GivenName:  identity.GetGivenName(),
			FamilyName: identity.GetFamilyName(),
			Motto:      identity.GetMotto(),
			Aliases:    append([]string(nil), identity.GetAliases()...),
		},
		Lang:          manifest.GetLang(),
		Runner:        safeBuildRunner(build),
		Status:        identity.GetStatus(),
		Kind:          manifest.GetKind(),
		Transport:     manifest.GetTransport(),
		Entrypoint:    safeArtifactBinary(artifacts),
		Architectures: append([]string(nil), manifest.GetPlatforms()...),
		BuildMain:     safeBuildMain(build),
	}, nil
}

func holonEntryFromInfo(info HolonInfo, dir string, sourceKind string) HolonEntry {
	entry := HolonEntry{
		Slug:       info.Slug,
		UUID:       info.UUID,
		Dir:        dir,
		SourceKind: sourceKind,
		Runner:     info.Runner,
		Transport:  info.Transport,
		Entrypoint: info.Entrypoint,
		Identity: identity.Identity{
			UUID:       info.UUID,
			GivenName:  info.Identity.GivenName,
			FamilyName: info.Identity.FamilyName,
			Motto:      info.Identity.Motto,
			Aliases:    append([]string(nil), info.Identity.Aliases...),
			Lang:       info.Lang,
			Status:     info.Status,
		},
		Manifest: &Manifest{
			Kind: info.Kind,
			Build: Build{
				Runner: info.Runner,
				Main:   info.BuildMain,
			},
			Artifacts: Artifacts{
				Binary: info.Entrypoint,
			},
		},
		Architectures: append([]string(nil), info.Architectures...),
		HasDist:       info.HasDist,
		HasSource:     info.HasSource,
	}
	if sourceKind == "package" {
		entry.PackageRoot = dir
	}
	return entry
}

func packageBinaryPath(dir string) (string, error) {
	archDir := filepath.Join(dir, "bin", runtime.GOOS+"_"+runtime.GOARCH)
	entries, err := os.ReadDir(archDir)
	if err != nil {
		return "", err
	}

	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		candidates = append(candidates, filepath.Join(archDir, entry.Name()))
	}
	if len(candidates) == 0 {
		return "", os.ErrNotExist
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

func safeBuildRunner(build *holonsv1.HolonManifest_Build) string {
	if build == nil {
		return ""
	}
	return build.GetRunner()
}

func safeBuildMain(build *holonsv1.HolonManifest_Build) string {
	if build == nil {
		return ""
	}
	return build.GetMain()
}

func safeArtifactBinary(artifacts *holonsv1.HolonManifest_Artifacts) string {
	if artifacts == nil {
		return ""
	}
	return artifacts.GetBinary()
}

func normalizeDialTarget(target string) string {
	trimmed := strings.TrimSpace(target)
	if !strings.Contains(trimmed, "://") {
		return trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}

	switch parsed.Scheme {
	case "tcp":
		host := parsed.Hostname()
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		port := parsed.Port()
		if port == "" {
			return trimmed
		}
		return host + ":" + port
	case "unix", "ws", "wss":
		return trimmed
	default:
		return trimmed
	}
}

func pathFromFileURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("holon URL %q is not a local file target", raw)
	}
	path := parsed.Path
	if parsed.Host != "" && parsed.Host != "localhost" {
		path = "//" + parsed.Host + path
	}
	if path == "" {
		return "", fmt.Errorf("holon URL %q has no path", raw)
	}
	return filepath.Clean(path), nil
}

func terminateCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		_ = cmd.Process.Kill()
		<-done
	}
}

func errorsIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "not exist"))
}
