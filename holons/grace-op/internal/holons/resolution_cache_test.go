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

func TestGlobalCacheUniqueResolutionPopulatesTier1(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{ref}}
	})

	expr := "alpha"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 {
		t.Fatalf("DiscoverRefs = %+v", result)
	}
	cached, ok := readResolutionGlobalEntry("alpha")
	if !ok {
		t.Fatal("global cache missed unique slug")
	}
	if cached.Info == nil || cached.Info.Slug != "alpha" {
		t.Fatalf("global cache ref = %+v, want alpha", cached)
	}
}

func TestGlobalCacheNoPopulationOnZeroMatches(t *testing.T) {
	root := setupResolutionCacheTest(t)
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{}
	})

	expr := "missing"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 0 {
		t.Fatalf("DiscoverRefs = %+v, want zero matches", result)
	}
	if _, ok := readResolutionGlobalEntry("missing"); ok {
		t.Fatal("global cache populated for zero matches")
	}
}

func TestGlobalCacheNoPopulationOnMultipleMatches(t *testing.T) {
	root := setupResolutionCacheTest(t)
	first := cacheTestRef(t, filepath.Join(root, "one"), "alpha", "alpha-one", "source")
	second := cacheTestRef(t, filepath.Join(root, "two"), "alpha", "alpha-two", "source")
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{first, second}}
	})

	expr := "alpha"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 2 {
		t.Fatalf("DiscoverRefs = %+v, want two matches", result)
	}
	if _, ok := readResolutionGlobalEntry("alpha"); ok {
		t.Fatal("global cache populated for multiple matches")
	}
}

func TestGlobalCacheHitShortCircuitsWalk(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	if err := writeResolutionGlobalEntry(ref); err != nil {
		t.Fatal(err)
	}
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Error: "walk should not run"}
	})

	expr := "alpha"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "alpha" {
		t.Fatalf("DiscoverRefs = %+v, want global hit", result)
	}
	if got := *calls; got != 0 {
		t.Fatalf("walk calls = %d, want 0", got)
	}
}

func TestGlobalCacheHonorsRequestedSpecifiers(t *testing.T) {
	root := setupResolutionCacheTest(t)
	installed := cacheTestRef(t, filepath.Join(os.Getenv("OPBIN"), "alpha.holon"), "alpha", "alpha-installed", "package")
	source := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-source", "source")
	if err := writeResolutionGlobalEntry(installed); err != nil {
		t.Fatal(err)
	}
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, specifiers int, _ int, _ int) sdkdiscover.DiscoverResult {
		if specifiers != sdkdiscover.SOURCE {
			t.Fatalf("fresh walk specifiers = 0x%02X, want SOURCE", specifiers)
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{source}}
	})

	expr := "alpha"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.UUID != "alpha-source" {
		t.Fatalf("DiscoverRefs SOURCE = %+v, want source ref", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1 because installed global entry must not satisfy SOURCE", got)
	}
}

func TestGlobalCacheStatInvalidationDropsEntry(t *testing.T) {
	root := setupResolutionCacheTest(t)
	stale := cacheTestRef(t, filepath.Join(root, "stale"), "alpha", "alpha-stale", "source")
	fresh := cacheTestRef(t, filepath.Join(root, "fresh"), "alpha", "alpha-fresh", "source")
	if err := writeResolutionGlobalEntry(stale); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "stale")); err != nil {
		t.Fatal(err)
	}
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{fresh}}
	})

	expr := "alpha"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.UUID != "alpha-fresh" {
		t.Fatalf("DiscoverRefs = %+v, want fresh alpha", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
	cached, ok := readResolutionGlobalEntry("alpha")
	if !ok || cached.Info.UUID != "alpha-fresh" {
		t.Fatalf("global cache after stale drop = ok:%v ref:%+v, want fresh", ok, cached)
	}
}

func TestGlobalCacheMarkerInvalidationIgnoresTier1(t *testing.T) {
	root := setupResolutionCacheTest(t)
	stale := cacheTestRef(t, filepath.Join(root, "stale"), "alpha", "alpha-stale", "source")
	fresh := cacheTestRef(t, filepath.Join(root, "fresh"), "alpha", "alpha-fresh", "source")
	if err := writeResolutionGlobalEntry(stale); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := TouchResolutionCacheDirty(); err != nil {
		t.Fatal(err)
	}
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{fresh}}
	})

	expr := "alpha"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.UUID != "alpha-fresh" {
		t.Fatalf("DiscoverRefs = %+v, want fresh alpha", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
}

func TestGlobalCacheLastWriteWinsOnSlugCollision(t *testing.T) {
	root := setupResolutionCacheTest(t)
	first := cacheTestRef(t, filepath.Join(root, "first"), "alpha", "alpha-first", "source")
	second := cacheTestRef(t, filepath.Join(root, "second"), "alpha", "alpha-second", "source")
	if err := writeResolutionGlobalEntry(first); err != nil {
		t.Fatal(err)
	}
	if err := writeResolutionGlobalEntry(second); err != nil {
		t.Fatal(err)
	}
	cached, ok := readResolutionGlobalEntry("alpha")
	if !ok || cached.Info.UUID != "alpha-second" {
		t.Fatalf("global cache = ok:%v ref:%+v, want second writer", ok, cached)
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

func TestResolutionCacheHidesInternalCompositeMembersFromSourceDiscovery(t *testing.T) {
	root := setupResolutionCacheTest(t)
	parentDir := filepath.Join(root, "parent")
	childDir := filepath.Join(parentDir, "holons", "node")
	writeRootHolonManifestMarker(t, parentDir)

	parentRef := cacheTestRef(t, parentDir, "parent", "parent-uuid", "source")
	childRef := cacheTestRef(t, childDir, "internal-node", "internal-node-uuid", "source")
	installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{parentRef, childRef}}
	})

	result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("DiscoverRefs error = %q", result.Error)
	}
	if got := len(result.Found); got != 1 {
		t.Fatalf("len(found) = %d, want 1: %+v", got, result.Found)
	}
	if result.Found[0].Info == nil || result.Found[0].Info.Slug != "parent" {
		t.Fatalf("found = %+v, want only parent", result.Found)
	}

	expr := "internal-node"
	result = DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("DiscoverRefs internal expression error = %q", result.Error)
	}
	if got := len(result.Found); got != 0 {
		t.Fatalf("internal slug len(found) = %d, want 0: %+v", got, result.Found)
	}
}

func TestResolutionCacheIgnoresStaleGlobalInternalMember(t *testing.T) {
	root := setupResolutionCacheTest(t)
	parentDir := filepath.Join(root, "parent")
	childDir := filepath.Join(parentDir, "holons", "node")
	writeRootHolonManifestMarker(t, parentDir)

	childRef := cacheTestRef(t, childDir, "internal-node", "internal-node-uuid", "source")
	if err := writeResolutionGlobalEntry(childRef); err != nil {
		t.Fatal(err)
	}
	calls := installResolutionDiscoverHook(t, func(_ *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{childRef}}
	})

	expr := "internal-node"
	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("DiscoverRefs error = %q", result.Error)
	}
	if got := len(result.Found); got != 0 {
		t.Fatalf("len(found) = %d, want 0: %+v", got, result.Found)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want stale global entry bypassed and walk retried", got)
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

func TestNoCacheBypassesReadButWritesResult(t *testing.T) {
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
	if stats := ResolutionCacheStatsSnapshot(); stats.Bypasses != 1 || stats.Writes != 1 || stats.Hits != 0 {
		t.Fatalf("stats = %+v, want bypass with write-through", stats)
	}
	refs, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE)
	if !ok || len(refs) != 1 || refs[0].Info.Slug != "fresh" {
		t.Fatalf("snapshot was not refreshed under no-cache: ok=%v refs=%+v", ok, refs)
	}
}

func TestNoCacheSlugRefreshWritesContextualSnapshot(t *testing.T) {
	root := setupResolutionCacheTest(t)
	freshRef := cacheTestRef(t, filepath.Join(root, "fresh"), "fresh", "fresh-uuid", "source")
	otherRef := cacheTestRef(t, filepath.Join(root, "other"), "other", "other-uuid", "source")
	expr := "fresh"
	SetResolutionCacheDisabled(true)
	calls := installResolutionDiscoverHook(t, func(expression *string, _ *string, _ int, limit int, _ int) sdkdiscover.DiscoverResult {
		if expression != nil {
			t.Fatalf("fresh slug refresh expression = %q, want nil contextual walk", *expression)
		}
		if limit != sdkdiscover.NO_LIMIT {
			t.Fatalf("fresh slug refresh limit = %d, want NO_LIMIT", limit)
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{freshRef, otherRef}}
	})

	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 || result.Found[0].Info.Slug != "fresh" {
		t.Fatalf("DiscoverRefs with no-cache slug = %+v, want fresh result", result)
	}
	if got := *calls; got != 1 {
		t.Fatalf("walk calls = %d, want 1", got)
	}
	refs, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE)
	if !ok || len(refs) != 2 {
		t.Fatalf("snapshot = ok:%v refs:%+v, want full contextual snapshot", ok, refs)
	}
	if _, ok := readResolutionGlobalEntry("fresh"); !ok {
		t.Fatal("no-cache slug refresh did not populate tier 1")
	}
}

func TestPathExpressionInvocationPopulatesTier1(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	expr := filepath.Join(root, "alpha")
	installResolutionDiscoverHook(t, func(expression *string, _ *string, _ int, _ int, _ int) sdkdiscover.DiscoverResult {
		if expression == nil || *expression != expr {
			t.Fatalf("expression = %v, want path expression %q", expression, expr)
		}
		return sdkdiscover.DiscoverResult{Found: []sdkdiscover.HolonRef{ref}}
	})

	result := DiscoverRefs(&expr, &root, sdkdiscover.SOURCE, 1, sdkdiscover.NO_TIMEOUT)
	if result.Error != "" || len(result.Found) != 1 {
		t.Fatalf("DiscoverRefs = %+v", result)
	}
	if _, ok := readResolutionGlobalEntry("alpha"); !ok {
		t.Fatal("path expression did not populate global cache")
	}
}

func TestResolutionCachePurgeDeletesAndSubsequentWalkRepopulates(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{ref}); err != nil {
		t.Fatal(err)
	}
	if err := writeResolutionGlobalEntry(ref); err != nil {
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

func TestPurgeCacheDeletesTier1AndTier2(t *testing.T) {
	root := setupResolutionCacheTest(t)
	ref := cacheTestRef(t, filepath.Join(root, "alpha"), "alpha", "alpha-uuid", "source")
	if err := writeResolutionSnapshot(root, sdkdiscover.SOURCE, []sdkdiscover.HolonRef{ref}); err != nil {
		t.Fatal(err)
	}
	if err := writeResolutionGlobalEntry(ref); err != nil {
		t.Fatal(err)
	}
	if err := PurgeResolutionCache(); err != nil {
		t.Fatal(err)
	}
	if _, ok := readResolutionGlobalEntry("alpha"); ok {
		t.Fatal("tier 1 survived purge")
	}
	if _, ok := readResolutionSnapshot(root, sdkdiscover.SOURCE); ok {
		t.Fatal("tier 2 survived purge")
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

func writeRootHolonManifestMarker(t *testing.T, dir string) {
	t.Helper()
	manifestDir := filepath.Join(dir, "api", "v1")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "holon.proto"), []byte("syntax = \"proto3\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
