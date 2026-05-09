package holons

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
)

var resolutionCacheBenchStates sync.Map

func BenchmarkResolutionCacheOpListSourceLayer(b *testing.B) {
	seedRoot := benchmarkSeedRoot(b)
	scenarios := []struct {
		name string
		root string
	}{
		{name: "slash", root: string(os.PathSeparator)},
		{name: "seedRoot", root: seedRoot},
		{name: "holonDir", root: filepath.Join(seedRoot, "examples", "hello-world", "gabriel-greeting-go")},
	}

	for _, scenario := range scenarios {
		if info, err := os.Stat(scenario.root); err != nil || !info.IsDir() {
			b.Run(scenario.name, func(b *testing.B) {
				b.Skipf("root %s is not available", scenario.root)
			})
			continue
		}
		if scenario.name == "slash" && os.Getenv("OP_RESOLUTION_CACHE_BENCH_SLASH") != "1" {
			b.Run(scenario.name, func(b *testing.B) {
				b.Skip("set OP_RESOLUTION_CACHE_BENCH_SLASH=1 to include the full filesystem / walk")
			})
			continue
		}
		b.Run(scenario.name, func(b *testing.B) {
			benchmarkResolutionCacheScenario(b, scenario.root, scenario.name == "seedRoot")
		})
	}
}

type resolutionCacheBenchState struct {
	oppath          string
	opbin           string
	coldDuration    time.Duration
	warmDuration    time.Duration
	noCacheDuration time.Duration
	stats           ResolutionCacheStats
	refs            int
	ratio           float64
}

func benchmarkResolutionCacheScenario(b *testing.B, root string, enforceRatio bool) {
	state := prepareResolutionCacheBenchState(b, root)
	if enforceRatio && state.ratio < 100 {
		b.Fatalf("warm/cold target missed: cold=%s warm=%s ratio=%.1fx", state.coldDuration, state.warmDuration, state.ratio)
	}

	oldOPPATH, hadOPPATH := os.LookupEnv("OPPATH")
	oldOPBIN, hadOPBIN := os.LookupEnv("OPBIN")
	_ = os.Setenv("OPPATH", state.oppath)
	_ = os.Setenv("OPBIN", state.opbin)
	ResetResolutionCacheOptions()
	b.Cleanup(func() {
		restoreBenchEnv("OPPATH", oldOPPATH, hadOPPATH)
		restoreBenchEnv("OPBIN", oldOPBIN, hadOPBIN)
		ResetResolutionCacheOptions()
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
		if result.Error != "" {
			b.Fatalf("discover: %s", result.Error)
		}
	}
	b.StopTimer()

	b.ReportMetric(float64(state.coldDuration.Microseconds())/1000, "cold_ms")
	b.ReportMetric(float64(state.warmDuration.Microseconds())/1000, "warm_ms")
	b.ReportMetric(float64(state.noCacheDuration.Microseconds())/1000, "no_cache_ms")
	b.ReportMetric(state.ratio, "cold_warm_ratio")
	b.ReportMetric(float64(state.stats.Hits), "hits")
	b.ReportMetric(float64(state.stats.Misses), "misses")
	b.ReportMetric(float64(state.stats.Bypasses), "bypasses")
	b.ReportMetric(float64(state.refs), "refs")
}

func prepareResolutionCacheBenchState(b *testing.B, root string) resolutionCacheBenchState {
	b.Helper()
	if value, ok := resolutionCacheBenchStates.Load(root); ok {
		return value.(resolutionCacheBenchState)
	}

	runtimeHome, err := os.MkdirTemp("", "op-resolution-cache-bench-")
	if err != nil {
		b.Fatalf("temp OPPATH: %v", err)
	}
	state := resolutionCacheBenchState{
		oppath: runtimeHome,
		opbin:  filepath.Join(runtimeHome, "bin"),
	}

	oldOPPATH, hadOPPATH := os.LookupEnv("OPPATH")
	oldOPBIN, hadOPBIN := os.LookupEnv("OPBIN")
	defer func() {
		restoreBenchEnv("OPPATH", oldOPPATH, hadOPPATH)
		restoreBenchEnv("OPBIN", oldOPBIN, hadOPBIN)
		ResetResolutionCacheOptions()
	}()
	_ = os.Setenv("OPPATH", state.oppath)
	_ = os.Setenv("OPBIN", state.opbin)
	ResetResolutionCacheOptions()
	ResetResolutionCacheStats()
	if err := PurgeResolutionCache(); err != nil {
		b.Fatalf("purge cache: %v", err)
	}

	start := time.Now()
	cold := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	state.coldDuration = time.Since(start)
	if cold.Error != "" {
		b.Fatalf("cold discover: %s", cold.Error)
	}

	start = time.Now()
	warm := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	state.warmDuration = time.Since(start)
	if warm.Error != "" {
		b.Fatalf("warm discover: %s", warm.Error)
	}

	SetResolutionCacheDisabled(true)
	start = time.Now()
	noCache := DiscoverRefs(nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	state.noCacheDuration = time.Since(start)
	if noCache.Error != "" {
		b.Fatalf("no-cache discover: %s", noCache.Error)
	}
	SetResolutionCacheDisabled(false)

	state.stats = ResolutionCacheStatsSnapshot()
	state.refs = len(warm.Found)
	if state.warmDuration > 0 {
		state.ratio = float64(state.coldDuration) / float64(state.warmDuration)
	}
	resolutionCacheBenchStates.Store(root, state)
	return state
}

func restoreBenchEnv(key, value string, present bool) {
	if present {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}

func benchmarkSeedRoot(b *testing.B) string {
	b.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
