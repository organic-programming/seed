package observability

import (
	"runtime"
	"runtime/debug"
)

const (
	metricBuildInfo               = "holon_build_info"
	metricProcessStartTimeSeconds = "holon_process_start_time_seconds"
	metricMemoryBytes             = "holon_memory_bytes"
	metricGoroutines              = "holon_goroutines"
)

func (o *Observability) initBaselineMetrics() {
	if o == nil || o.registry == nil {
		return
	}
	version, commit := buildInfoLabels()
	o.registry.Gauge(metricBuildInfo,
		"Holon build information.",
		map[string]string{
			"version": version,
			"lang":    "go",
			"commit":  commit,
		}).Set(1)
	o.registry.Gauge(metricProcessStartTimeSeconds,
		"Unix process start time in seconds.",
		nil).Set(float64(o.startWall.UnixNano()) / 1e9)
	o.registry.refresh = o.refreshRuntimeMetrics
	o.refreshRuntimeMetrics()
}

func (o *Observability) refreshRuntimeMetrics() {
	if o == nil || o.registry == nil {
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	o.registry.Gauge(metricMemoryBytes,
		"Process memory bytes by class.",
		map[string]string{"class": "rss"}).Set(float64(mem.Sys))
	o.registry.Gauge(metricMemoryBytes,
		"Process memory bytes by class.",
		map[string]string{"class": "heap"}).Set(float64(mem.HeapAlloc))
	o.registry.Gauge(metricMemoryBytes,
		"Process memory bytes by class.",
		map[string]string{"class": "stack"}).Set(float64(mem.StackInuse))
	o.registry.Gauge(metricGoroutines,
		"Live Go goroutines.",
		nil).Set(float64(runtime.NumGoroutine()))
}

func buildInfoLabels() (version, commit string) {
	version = "unknown"
	commit = "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				commit = setting.Value
				break
			}
		}
	}
	return version, commit
}
