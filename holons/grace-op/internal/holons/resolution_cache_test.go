package holons

import (
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
)

func TestResolutionCacheRoundTripWriteRead(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")

	if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{ref}); err != nil {
		t.Fatalf("writeResolutionSnapshot: %v", err)
	}
	refs, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE)
	if !ok {
		t.Fatal("readResolutionSnapshot missed written snapshot")
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	if refs[0].Info == nil || refs[0].Info.Slug != "alpha" || refs[0].Info.SourceKind != "source" {
		t.Fatalf("cached ref info = %+v, want slug/source kind preserved", refs[0].Info)
	}
}

func TestResolutionCacheFullHitAvoidsWalk(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{ref}}
	})

	first := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if first.Error != "" || len(first.Found) != 1 {
		t.Fatalf("first DiscoverRefs = %+v", first)
	}
	second := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if second.Error != "" || len(second.Found) != 1 {
		t.Fatalf("second DiscoverRefs = %+v", second)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
	if stats := ResolutionCacheStatsSnapshot(); stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("stats = %+v, want one hit and one miss", stats)
	}
}

func TestResolutionCachePerEntryInvalidationFallsBackToWalk(t *testing.T) {
	root := setupResolutionCacheTest(t)
	firstRef := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	secondRef := cacheTestRef(t, filepath.Join(root, "beta"), "beta", "beta-uuid", "source")
	calls := 0
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		calls++
		if calls == 1 {
			return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{firstRef}}
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{secondRef}}
	})

	_ = DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if err := os.RemoveAll(filepath.Join(root, "alpha")); err != nil {
		t.Fatal(err)
	}

	result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "beta" {
		t.Fatalf("DiscoverRefs after stale entry = %+v, want fresh beta result", result)
	}
	if calls != 2 {
		t.Fatalf("walk calls = %d, want 2", calls)
	}
}

func TestResolutionCacheMtimeChangeFallsBackToWalk(t *testing.T) {
	root := setupResolutionCacheTest(t)
	target := filepath.Join(root, "alpha")
	firstRef := cacheTestRef(t, target, "alpha", "alpha-uuid", "source")
	secondRef := cacheTestRef(t, filepath.Join(root, "beta"), "beta", "beta-uuid", "source")
	calls := 0
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		calls++
		if calls == 1 {
			return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{firstRef}}
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{secondRef}}
	})

	_ = DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	next := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(target, next, next); err != nil {
		t.Fatal(err)
	}

	result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "beta" {
		t.Fatalf("DiscoverRefs after mtime change = %+v, want fresh beta result", result)
	}
	if calls != 2 {
		t.Fatalf("walk calls = %d, want 2", calls)
	}
}

func TestResolutionCacheMarkerInvalidationIgnoresSnapshot(t *testing.T) {
	root := setupResolutionCacheTest(t)
	firstRef := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	secondRef := cacheTestRef(t, filepath.Join(root, "beta"), "beta", "beta-uuid", "source")
	calls := 0
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		calls++
		if calls == 1 {
			return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{firstRef}}
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{secondRef}}
	})

	_ = DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	time.Sleep(10 * time.Millisecond)
	if err := TouchResolutionCacheDirty(); err != nil {
		t.Fatal(err)
	}

	result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "beta" {
		t.Fatalf("DiscoverRefs after marker = %+v, want fresh beta result", result)
	}
	if calls != 2 {
		t.Fatalf("walk calls = %d, want 2", calls)
	}
}

func TestResolutionCacheNoCacheBypassesReadAndWrite(t *testing.T) {
	root := setupResolutionCacheTest(t)
	cachedRef := cacheTestRef(t, filepath.Join(root, "cached"), "cached", "cached-uuid", "source")
	freshRef := cacheTestRef(t, filepath.Join(root, "fresh"), "fresh", "fresh-uuid", "source")
	if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{cachedRef}); err != nil {
		t.Fatal(err)
	}
	ResetResolutionCacheStats()
	SetResolutionCacheDisabled(true)
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{freshRef}}
	})

	result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "fresh" {
		t.Fatalf("DiscoverRefs with no-cache = %+v, want fresh result", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
	if stats := ResolutionCacheStatsSnapshot(); stats.Bypasses != 1 || stats.Writes != 0 || stats.Hits != 0 {
		t.Fatalf("stats = %+v, want bypass without read/write", stats)
	}
	refs, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE)
	if !ok || len(refs) != 1 || refs[0].Info.Slug != "cached" {
		t.Fatalf("snapshot changed under no-cache: ok=%v refs=%+v", ok, refs)
	}
}

func TestResolutionCachePurgeDeletesAndSubsequentWalkRepopulates(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{ref}); err != nil {
		t.Fatal(err)
	}
	if err := TouchResolutionCacheDirty(); err != nil {
		t.Fatal(err)
	}
	if err := PurgeResolutionCache(); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(resolutionCacheDir()); err == nil && len(entries) > 0 {
		t.Fatalf("resolution cache dir still has entries after purge: %v", entries)
	}

	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{ref}}
	})
	result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 {
		t.Fatalf("DiscoverRefs after purge = %+v", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
	if _, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE); !ok {
		t.Fatal("snapshot was not repopulated after purge")
	}
}

func TestResolutionCacheNoNegativeCaching(t *testing.T) {
	root := setupResolutionCacheTest(t)
	alpha := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	beta := cacheTestRef(t, filepath.Join(root, "beta"), "beta", "beta-uuid", "source")
	if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{alpha}); err != nil {
		t.Fatal(err)
	}
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{alpha, beta}}
	})

	expr := "beta"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "beta" {
		t.Fatalf("DiscoverRefs absent from snapshot = %+v, want fresh beta", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
}

func TestResolutionCacheSpecifierIsolation(t *testing.T) {
	root := setupResolutionCacheTest(t)
	source := cacheTestRef(t, filepath.Join(root, "source"), "source", "source-uuid", "source")
	installed := cacheTestRef(t, filepath.Join(root, "installed.holon"), "installed", "installed-uuid", "package")
	callsBySpec := map[int]int{}
	installResolutionDiscoverHook(t, func(_ *string, _ *string, specifiers int, _ int, _ int) sdkdiscover.DiscoverResult {
		callsBySpec[specifiers]++
		if specifiers == sdkdiscover.INSTALLED {
			return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{installed}}
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{source}}
	})

	for i := 0; i < 2; i++ {
		sourceResult := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
		if sourceResult.Error != "" || sourceResult.Found[0].Info.Slug != "source" {
			t.Fatalf("source result = %+v", sourceResult)
		}
		installedResult := DiscoverRefs(nil, &root, sdkdiscover.INSTALLED, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
		if installedResult.Error != "" || installedResult.Found[0].Info.Slug != "installed" {
			t.Fatalf("installed result = %+v", installedResult)
		}
	}
	if callsBySpec[sdkdiscover.SOURCE] != 1 || callsBySpec[sdkdiscover.INSTALLED] != 1 {
		t.Fatalf("calls by specifier = %v, want independent one-time fills", callsBySpec)
	}
}

func TestResolutionCacheConcurrentWritesAreAtomic(t *testing.T) {
	root := setupResolutionCacheTest(t)
	alpha := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	beta := cacheTestRef(t, filepath.Join(root, "beta"), "beta", "beta-uuid", "source")

	var wg sync.WaitGroup
	for _, ref := range []sdkdiscover.HolonRef{alpha, beta} {
		wg.Add(1)
		go func(ref sdkdiscover.HolonRef) {
			defer wg.Done()
			if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{ref}); err != nil {
				t.Errorf("writeResolutionSnapshot: %v", err)
			}
		}(ref)
	}
	wg.Wait()

	refs, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE)
	if !ok {
		t.Fatal("snapshot unreadable after concurrent writes")
	}
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	switch refs[0].Info.Slug {
	case "alpha", "beta":
	default:
		t.Fatalf("unexpected winning snapshot: %+v", refs[0])
	}
}

func setupResolutionCacheTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if canonical, err := canonicalResolutionRoot(&root); err == nil {
		root = canonical
	}
	t.Setenv("OPPATH", filepath.Join(root, ".op"))
	t.Setenv("OPBIN", filepath.Join(root, ".op", "bin"))
	ResetResolutionCacheOptions()
	ResetResolutionCacheStats()
	t.Cleanup(func() {
		ResetResolutionCacheOptions()
		ResetResolutionCacheStats()
	})
	return root
}

func installResolutionDiscoverHook(t *testing.T, fn func(*string, *string, int, int, int) sdkdiscover.DiscoverResult) *int {
	t.Helper()
	calls := 0
	previous := resolutionDiscover
	resolutionDiscover = func(expression *string, root *string, specifiers int, limit int, timeout int) sdkdiscover.DiscoverResult {
		calls++
		return fn(expression, root, specifiers, limit, timeout)
	}
	t.Cleanup(func() {
		resolutionDiscover = previous
	})
	return &calls
}

func cacheTestRef(t *testing.T, dir string, slug string, uuid string, sourceKind string) sdkdiscover.HolonRef {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return sdkdiscover.HolonRef{
		URL: (&url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}).String(),
		Info: &sdkdiscover.HolonInfo{
			Slug:       slug,
			UUID:       uuid,
			Lang:       "go",
			Runner:     "go-module",
			Status:     "draft",
			Kind:       "native",
			SourceKind: sourceKind,
			Identity: sdkdiscover.IdentityInfo{
				GivenName:  slug,
				FamilyName: "Holon",
				Aliases:    []string{slug + "-alias"},
			},
		},
	}
}
