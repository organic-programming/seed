package holons

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	openv "github.com/organic-programming/grace-op/internal/env"
)

const resolutionCacheVersion = 1
const resolutionGlobalCacheName = "global.json"

var (
	resolutionCacheMu       sync.RWMutex
	resolutionCacheDisabled bool

	resolutionDiscover = func(expression *string, root *string, specifiers int, limit int, timeout int) sdkdiscover.DiscoverResult {
		return sdkdiscover.Discover(sdkdiscover.LOCAL, expression, root, specifiers, limit, timeout)
	}

	resolutionCacheHitCount    atomic.Uint64
	resolutionCacheMissCount   atomic.Uint64
	resolutionCacheWriteCount  atomic.Uint64
	resolutionCacheBypassCount atomic.Uint64
	resolutionCachePurgeCount  atomic.Uint64
	resolutionCacheWriteSeq    atomic.Uint64
)

type ResolutionCacheStats struct {
	Hits     uint64
	Misses   uint64
	Writes   uint64
	Bypasses uint64
	Purges   uint64
}

type resolutionSnapshot struct {
	Version    int                       `json:"version"`
	Root       string                    `json:"root"`
	Specifiers string                    `json:"specifiers"`
	ScannedAt  string                    `json:"scanned_at"`
	Entries    []resolutionSnapshotEntry `json:"entries"`
}

type resolutionSnapshotEntry struct {
	URL           string               `json:"url"`
	Info          *resolutionCacheInfo `json:"info,omitempty"`
	TargetPath    string               `json:"target_path"`
	TargetMtimeNS int64                `json:"target_mtime_ns"`
}

type resolutionGlobalCache struct {
	Version int                                `json:"version"`
	Entries map[string]resolutionSnapshotEntry `json:"entries"`
}

type resolutionCacheInfo struct {
	Slug          string                   `json:"slug"`
	UUID          string                   `json:"uuid"`
	Identity      sdkdiscover.IdentityInfo `json:"identity"`
	Lang          string                   `json:"lang"`
	Runner        string                   `json:"runner"`
	Status        string                   `json:"status"`
	Kind          string                   `json:"kind"`
	Transport     string                   `json:"transport"`
	Entrypoint    string                   `json:"entrypoint"`
	Architectures []string                 `json:"architectures"`
	HasDist       bool                     `json:"has_dist"`
	HasSource     bool                     `json:"has_source"`
	BuildMain     string                   `json:"build_main,omitempty"`
	SourceKind    string                   `json:"source_kind,omitempty"`
}

func ResetResolutionCacheOptions() {
	SetResolutionCacheDisabled(false)
}

func SetResolutionCacheDisabled(disabled bool) {
	resolutionCacheMu.Lock()
	defer resolutionCacheMu.Unlock()
	resolutionCacheDisabled = disabled
}

func ResolutionCacheDisabled() bool {
	resolutionCacheMu.RLock()
	defer resolutionCacheMu.RUnlock()
	return resolutionCacheDisabled
}

func ResolutionCacheStatsSnapshot() ResolutionCacheStats {
	return ResolutionCacheStats{
		Hits:     resolutionCacheHitCount.Load(),
		Misses:   resolutionCacheMissCount.Load(),
		Writes:   resolutionCacheWriteCount.Load(),
		Bypasses: resolutionCacheBypassCount.Load(),
		Purges:   resolutionCachePurgeCount.Load(),
	}
}

func ResetResolutionCacheStats() {
	resolutionCacheHitCount.Store(0)
	resolutionCacheMissCount.Store(0)
	resolutionCacheWriteCount.Store(0)
	resolutionCacheBypassCount.Store(0)
	resolutionCachePurgeCount.Store(0)
}

func PurgeResolutionCache() error {
	resolutionCachePurgeCount.Add(1)
	return os.RemoveAll(resolutionCacheDir())
}

func TouchResolutionCacheDirty() error {
	dir := resolutionCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, ".dirty")
	now := time.Now()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if closeErr := file.Close(); closeErr != nil {
		return closeErr
	}
	return os.Chtimes(path, now, now)
}

func discoverRefsWithResolutionCache(expression *string, root *string, specifiers int, limit int, timeout int) sdkdiscover.DiscoverResult {
	if limit < 0 || specifiers < 0 || specifiers > sdkdiscover.ALL {
		resolutionCacheBypassCount.Add(1)
		return discoverRefsBypassWithWrites(expression, root, specifiers, limit, timeout)
	}
	specifiers = normalizeResolutionSpecifiers(specifiers)

	if resolutionCacheBypassExpression(expression) {
		resolutionCacheBypassCount.Add(1)
		return discoverRefsBypassWithWrites(expression, root, specifiers, limit, timeout)
	}

	canonicalRoot, err := canonicalResolutionRoot(root)
	if err != nil {
		resolutionCacheBypassCount.Add(1)
		return discoverRefsBypassWithWrites(expression, root, specifiers, limit, timeout)
	}
	canonicalRootPtr := canonicalRoot

	noCache := ResolutionCacheDisabled()
	if noCache {
		resolutionCacheBypassCount.Add(1)
	} else if slug, ok := resolutionSlugExpression(expression); ok {
		if ref, ok := readResolutionGlobalEntry(slug); ok {
			if !isInternalSourceRef(canonicalRoot, ref) && resolutionRefMatchesSpecifiers(canonicalRoot, ref, specifiers) {
				resolutionCacheHitCount.Add(1)
				return sdkdiscover.DiscoverResult{Found: limitResolutionRefs([]sdkdiscover.HolonRef{ref}, limit)}
			}
		}
	} else if refs, ok := readResolutionSnapshot(canonicalRoot, specifiers); ok {
		refs = filterInternalSourceRefs(canonicalRoot, refs)
		filtered := filterResolutionRefs(refs, expression)
		if expression == nil || len(filtered) > 0 {
			resolutionCacheHitCount.Add(1)
			return sdkdiscover.DiscoverResult{Found: limitResolutionRefs(filtered, limit)}
		}
	}

	if !noCache {
		if refs, ok := readResolutionSnapshot(canonicalRoot, specifiers); ok {
			refs = filterInternalSourceRefs(canonicalRoot, refs)
			filtered := filterResolutionRefs(refs, expression)
			if expression == nil || len(filtered) > 0 {
				resolutionCacheHitCount.Add(1)
				return sdkdiscover.DiscoverResult{Found: limitResolutionRefs(filtered, limit)}
			}
		}
	}

	resolutionCacheMissCount.Add(1)
	fresh := resolutionDiscover(nil, &canonicalRootPtr, specifiers, sdkdiscover.NO_LIMIT, timeout)
	if fresh.Error != "" {
		return fresh
	}
	fresh.Found = filterInternalSourceRefs(canonicalRoot, fresh.Found)
	_ = writeResolutionSnapshot(canonicalRoot, specifiers, fresh.Found)
	filtered := filterResolutionRefs(fresh.Found, expression)
	if _, ok := resolutionSlugExpression(expression); ok && len(filtered) == 1 {
		_ = writeResolutionGlobalEntry(filtered[0])
	}
	return sdkdiscover.DiscoverResult{Found: limitResolutionRefs(filtered, limit)}
}

func discoverRefsBypassWithWrites(expression *string, root *string, specifiers int, limit int, timeout int) sdkdiscover.DiscoverResult {
	if specifiers >= 0 && specifiers <= sdkdiscover.ALL {
		normalizedSpecifiers := normalizeResolutionSpecifiers(specifiers)
		if _, ok := resolutionSlugExpression(expression); ok {
			if canonicalRoot, err := canonicalResolutionRoot(root); err == nil {
				canonicalRootPtr := canonicalRoot
				fresh := resolutionDiscover(nil, &canonicalRootPtr, normalizedSpecifiers, sdkdiscover.NO_LIMIT, timeout)
				if fresh.Error != "" {
					return fresh
				}
				_ = writeResolutionSnapshot(canonicalRoot, normalizedSpecifiers, fresh.Found)
				filtered := filterResolutionRefs(fresh.Found, expression)
				if len(filtered) == 1 {
					_ = writeResolutionGlobalEntry(filtered[0])
				}
				return sdkdiscover.DiscoverResult{Found: limitResolutionRefs(filtered, limit)}
			}
		}
	}

	fresh := resolutionDiscover(expression, root, specifiers, limit, timeout)
	if fresh.Error != "" {
		return fresh
	}
	if specifiers < 0 || specifiers > sdkdiscover.ALL {
		return fresh
	}
	specifiers = normalizeResolutionSpecifiers(specifiers)
	if resolutionCacheBypassExpression(expression) {
		for _, ref := range fresh.Found {
			if ref.Info != nil && strings.TrimSpace(ref.Info.Slug) != "" {
				_ = writeResolutionGlobalEntry(ref)
				break
			}
		}
		return fresh
	}

	canonicalRoot, err := canonicalResolutionRoot(root)
	if err != nil {
		return fresh
	}
	_ = writeResolutionSnapshot(canonicalRoot, specifiers, fresh.Found)
	filtered := filterResolutionRefs(fresh.Found, expression)
	if _, ok := resolutionSlugExpression(expression); ok && len(filtered) == 1 {
		_ = writeResolutionGlobalEntry(filtered[0])
	}
	return fresh
}

func normalizeResolutionSpecifiers(specifiers int) int {
	if specifiers == 0 {
		return sdkdiscover.ALL
	}
	return specifiers
}

func canonicalResolutionRoot(root *string) (string, error) {
	resolved := openv.Root()
	if root != nil {
		resolved = strings.TrimSpace(*root)
	}
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("root cannot be empty")
	}
	absRoot, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	if evalRoot, evalErr := filepath.EvalSymlinks(absRoot); evalErr == nil {
		absRoot = evalRoot
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root %q is not a directory", resolved)
	}
	return filepath.Clean(absRoot), nil
}

func resolutionCacheBypassExpression(expression *string) bool {
	if expression == nil {
		return false
	}
	trimmed := strings.TrimSpace(*expression)
	if trimmed == "" {
		return false
	}
	if _, _, err := net.SplitHostPort(trimmed); err == nil {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err == nil {
		switch strings.ToLower(parsed.Scheme) {
		case "file", "tcp", "unix", "ws", "wss", "http", "https":
			return true
		}
	}
	return filepath.IsAbs(trimmed) ||
		strings.HasPrefix(trimmed, ".") ||
		strings.Contains(trimmed, "/") ||
		strings.Contains(trimmed, `\`) ||
		strings.HasSuffix(strings.ToLower(trimmed), ".holon")
}

func resolutionRefMatchesSpecifiers(root string, ref sdkdiscover.HolonRef, specifiers int) bool {
	if ref.Info == nil {
		return true
	}
	switch strings.TrimSpace(strings.ToLower(ref.Info.SourceKind)) {
	case "source":
		return specifiers&sdkdiscover.SOURCE != 0
	case "package":
		path, err := pathFromRefURL(ref.URL)
		if err != nil {
			return specifiers&(sdkdiscover.BUILT|sdkdiscover.INSTALLED|sdkdiscover.CACHED|sdkdiscover.SIBLINGS|sdkdiscover.CWD) != 0
		}
		cleanPath := filepath.Clean(path)
		switch {
		case isWithinBase(openv.OPBIN(), cleanPath):
			return specifiers&sdkdiscover.INSTALLED != 0
		case isWithinBase(filepath.Join(root, ".op", "build"), cleanPath):
			return specifiers&sdkdiscover.BUILT != 0
		case isWithinBase(openv.CacheDir(), cleanPath):
			return specifiers&sdkdiscover.CACHED != 0
		default:
			return specifiers&(sdkdiscover.BUILT|sdkdiscover.INSTALLED|sdkdiscover.CACHED|sdkdiscover.SIBLINGS|sdkdiscover.CWD) != 0
		}
	default:
		return true
	}
}

func resolutionSlugExpression(expression *string) (string, bool) {
	if expression == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(*expression)
	if trimmed == "" || resolutionCacheBypassExpression(expression) {
		return "", false
	}
	return trimmed, true
}

func readResolutionSnapshot(root string, specifiers int) ([]sdkdiscover.HolonRef, bool) {
	path := resolutionSnapshotPath(root, specifiers)
	snapshotInfo, err := os.Stat(path)
	if err != nil || snapshotInfo.IsDir() {
		return nil, false
	}
	if markerInfo, markerErr := os.Stat(filepath.Join(resolutionCacheDir(), ".dirty")); markerErr == nil {
		if snapshotInfo.ModTime().Before(markerInfo.ModTime()) {
			return nil, false
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var snapshot resolutionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, false
	}
	if snapshot.Version != resolutionCacheVersion ||
		filepath.Clean(snapshot.Root) != filepath.Clean(root) ||
		snapshot.Specifiers != resolutionSpecifierString(specifiers) {
		return nil, false
	}

	refs := make([]sdkdiscover.HolonRef, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if !resolutionSnapshotEntryClean(entry) {
			return nil, false
		}
		refs = append(refs, sdkdiscover.HolonRef{
			URL:  entry.URL,
			Info: entry.Info.toHolonInfo(),
		})
	}
	return refs, true
}

func writeResolutionSnapshot(root string, specifiers int, refs []sdkdiscover.HolonRef) error {
	dir := resolutionCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	entries := make([]resolutionSnapshotEntry, 0, len(refs))
	for _, ref := range refs {
		entry, ok := resolutionSnapshotEntryFromRef(ref)
		if ok {
			entries = append(entries, entry)
		}
	}

	snapshot := resolutionSnapshot{
		Version:    resolutionCacheVersion,
		Root:       root,
		Specifiers: resolutionSpecifierString(specifiers),
		ScannedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		Entries:    entries,
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	path := resolutionSnapshotPath(root, specifiers)
	tmpPath := fmt.Sprintf("%s.%d.%d.tmp", path, os.Getpid(), resolutionCacheWriteSeq.Add(1))
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	resolutionCacheWriteCount.Add(1)
	return nil
}

func readResolutionGlobalEntry(slug string) (sdkdiscover.HolonRef, bool) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return sdkdiscover.HolonRef{}, false
	}
	path := resolutionGlobalCachePath()
	cacheInfo, err := os.Stat(path)
	if err != nil || cacheInfo.IsDir() {
		return sdkdiscover.HolonRef{}, false
	}
	if markerInfo, markerErr := os.Stat(filepath.Join(resolutionCacheDir(), ".dirty")); markerErr == nil {
		if cacheInfo.ModTime().Before(markerInfo.ModTime()) {
			return sdkdiscover.HolonRef{}, false
		}
	}

	cache, ok := readResolutionGlobalCache()
	if !ok {
		return sdkdiscover.HolonRef{}, false
	}
	entry, ok := cache.Entries[slug]
	if !ok {
		return sdkdiscover.HolonRef{}, false
	}
	if !resolutionSnapshotEntryClean(entry) {
		_ = deleteResolutionGlobalEntry(slug)
		return sdkdiscover.HolonRef{}, false
	}
	return sdkdiscover.HolonRef{
		URL:  entry.URL,
		Info: entry.Info.toHolonInfo(),
	}, true
}

func writeResolutionGlobalEntry(ref sdkdiscover.HolonRef) error {
	entry, ok := resolutionSnapshotEntryFromRef(ref)
	if !ok || entry.Info == nil {
		return nil
	}
	slug := strings.TrimSpace(entry.Info.Slug)
	if slug == "" {
		return nil
	}

	cache, ok := readResolutionGlobalCache()
	if !ok || cache.Entries == nil {
		cache = resolutionGlobalCache{
			Version: resolutionCacheVersion,
			Entries: map[string]resolutionSnapshotEntry{},
		}
	}
	cache.Version = resolutionCacheVersion
	cache.Entries[slug] = entry
	return writeResolutionGlobalCache(cache)
}

func deleteResolutionGlobalEntry(slug string) error {
	cache, ok := readResolutionGlobalCache()
	if !ok || cache.Entries == nil {
		return nil
	}
	delete(cache.Entries, strings.TrimSpace(slug))
	return writeResolutionGlobalCache(cache)
}

func readResolutionGlobalCache() (resolutionGlobalCache, bool) {
	data, err := os.ReadFile(resolutionGlobalCachePath())
	if err != nil {
		return resolutionGlobalCache{}, false
	}
	var cache resolutionGlobalCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return resolutionGlobalCache{}, false
	}
	if cache.Version != resolutionCacheVersion {
		return resolutionGlobalCache{}, false
	}
	if cache.Entries == nil {
		cache.Entries = map[string]resolutionSnapshotEntry{}
	}
	return cache, true
}

func writeResolutionGlobalCache(cache resolutionGlobalCache) error {
	dir := resolutionCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if cache.Entries == nil {
		cache.Entries = map[string]resolutionSnapshotEntry{}
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := resolutionGlobalCachePath()
	tmpPath := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	resolutionCacheWriteCount.Add(1)
	return nil
}

func resolutionSnapshotEntryFromRef(ref sdkdiscover.HolonRef) (resolutionSnapshotEntry, bool) {
	if ref.Error != "" {
		return resolutionSnapshotEntry{}, false
	}
	targetPath, err := resolutionTargetPath(ref)
	if err != nil || strings.TrimSpace(targetPath) == "" {
		return resolutionSnapshotEntry{}, false
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return resolutionSnapshotEntry{}, false
	}
	return resolutionSnapshotEntry{
		URL:           ref.URL,
		Info:          resolutionCacheInfoFromHolonInfo(ref.Info),
		TargetPath:    targetPath,
		TargetMtimeNS: info.ModTime().UnixNano(),
	}, true
}

func resolutionSnapshotEntryClean(entry resolutionSnapshotEntry) bool {
	if strings.TrimSpace(entry.TargetPath) == "" {
		return false
	}
	info, err := os.Stat(entry.TargetPath)
	if err != nil {
		return false
	}
	return info.ModTime().UnixNano() == entry.TargetMtimeNS
}

func resolutionTargetPath(ref sdkdiscover.HolonRef) (string, error) {
	path, err := pathFromRefURL(ref.URL)
	if err != nil {
		return "", err
	}
	if ref.Info != nil && strings.TrimSpace(ref.Info.SourceKind) == "binary" {
		return filepath.Clean(path), nil
	}
	return filepath.Clean(path), nil
}

func resolutionCacheInfoFromHolonInfo(info *sdkdiscover.HolonInfo) *resolutionCacheInfo {
	if info == nil {
		return nil
	}
	return &resolutionCacheInfo{
		Slug:          info.Slug,
		UUID:          info.UUID,
		Identity:      info.Identity,
		Lang:          info.Lang,
		Runner:        info.Runner,
		Status:        info.Status,
		Kind:          info.Kind,
		Transport:     info.Transport,
		Entrypoint:    info.Entrypoint,
		Architectures: append([]string(nil), info.Architectures...),
		HasDist:       info.HasDist,
		HasSource:     info.HasSource,
		BuildMain:     info.BuildMain,
		SourceKind:    info.SourceKind,
	}
}

func (info *resolutionCacheInfo) toHolonInfo() *sdkdiscover.HolonInfo {
	if info == nil {
		return nil
	}
	return &sdkdiscover.HolonInfo{
		Slug:          info.Slug,
		UUID:          info.UUID,
		Identity:      info.Identity,
		Lang:          info.Lang,
		Runner:        info.Runner,
		Status:        info.Status,
		Kind:          info.Kind,
		Transport:     info.Transport,
		Entrypoint:    info.Entrypoint,
		Architectures: append([]string(nil), info.Architectures...),
		HasDist:       info.HasDist,
		HasSource:     info.HasSource,
		BuildMain:     info.BuildMain,
		SourceKind:    info.SourceKind,
	}
}

func filterResolutionRefs(refs []sdkdiscover.HolonRef, expression *string) []sdkdiscover.HolonRef {
	if expression == nil {
		return append([]sdkdiscover.HolonRef(nil), refs...)
	}
	needle := strings.TrimSpace(*expression)
	if needle == "" {
		return []sdkdiscover.HolonRef{}
	}
	filtered := make([]sdkdiscover.HolonRef, 0, len(refs))
	for _, ref := range refs {
		if resolutionRefMatches(ref, needle) {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

func filterInternalSourceRefs(root string, refs []sdkdiscover.HolonRef) []sdkdiscover.HolonRef {
	filtered := refs[:0]
	for _, ref := range refs {
		if !isInternalSourceRef(root, ref) {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

func isInternalSourceRef(root string, ref sdkdiscover.HolonRef) bool {
	if ref.Info == nil || strings.TrimSpace(ref.Info.SourceKind) != "source" {
		return false
	}
	refPath, err := pathFromRefURL(ref.URL)
	if err != nil {
		return false
	}
	return isInsideInternalHolonsDir(root, refPath)
}

func resolutionRefMatches(ref sdkdiscover.HolonRef, expression string) bool {
	if ref.Info != nil {
		if strings.TrimSpace(ref.Info.Slug) == expression {
			return true
		}
		if uuid := strings.TrimSpace(ref.Info.UUID); uuid != "" && strings.HasPrefix(uuid, expression) {
			return true
		}
		for _, alias := range ref.Info.Identity.Aliases {
			if strings.TrimSpace(alias) == expression {
				return true
			}
		}
	}
	refPath, err := pathFromRefURL(ref.URL)
	if err != nil {
		return false
	}
	base := strings.TrimSuffix(filepath.Base(refPath), ".holon")
	return base == expression
}

func limitResolutionRefs(refs []sdkdiscover.HolonRef, limit int) []sdkdiscover.HolonRef {
	if limit <= 0 || len(refs) <= limit {
		return refs
	}
	return refs[:limit]
}

func resolutionCacheDir() string {
	return filepath.Join(openv.OPPATH(), "resolutions")
}

func resolutionSnapshotPath(root string, specifiers int) string {
	return filepath.Join(resolutionCacheDir(), resolutionSnapshotHash(root, specifiers)+".json")
}

func resolutionGlobalCachePath() string {
	return filepath.Join(resolutionCacheDir(), resolutionGlobalCacheName)
}

func resolutionSnapshotHash(root string, specifiers int) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root) + "|" + resolutionSpecifierString(specifiers)))
	return hex.EncodeToString(sum[:])[:16]
}

func resolutionSpecifierString(specifiers int) string {
	return fmt.Sprintf("0x%02X", specifiers)
}
