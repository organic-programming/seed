package holons

import "testing"

func TestDefaultTargetRespectsManifestAndNativeHost(t *testing.T) {
	for _, tc := range []struct {
		name, kind, runner, defaultTarget, defaultMode, explicitTarget, explicitMode string
		windowsRecipe                                                                bool
		wantTarget, wantMode                                                         string
	}{
		{"macos desktop default", KindComposite, RunnerRecipe, "macos", "debug", "", "", true, "macos", "debug"},
		{"no target default", KindComposite, RunnerRecipe, "", "release", "", "", true, canonicalRuntimeTarget(), "release"},
		{"native target with explicit mode", KindComposite, RunnerRecipe, "", "debug", "", "release", true, canonicalRuntimeTarget(), "release"},
		{"explicit macos", KindComposite, RunnerRecipe, "macos", "debug", "macos", "", true, "macos", "debug"},
		{"explicit linux", KindComposite, RunnerRecipe, "macos", "debug", "linux", "", true, "linux", "debug"},
		{"explicit all", KindComposite, RunnerRecipe, "macos", "debug", "all", "", true, "all", "debug"},
		{"explicit mode", KindComposite, RunnerRecipe, "macos", "debug", "", "release", true, "macos", "release"},
		{"no windows recipe", KindComposite, RunnerRecipe, "macos", "debug", "", "", false, "macos", "debug"},
		{"aggregate default", KindComposite, RunnerRecipe, "all", "debug", "", "", true, "all", "debug"},
		{"mobile default", KindComposite, RunnerRecipe, "android", "debug", "", "", true, "android", "debug"},
		{"native holon", KindNative, RunnerCargo, "macos", "debug", "", "", true, "macos", "debug"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := &LoadedManifest{Manifest: Manifest{
				Kind: tc.kind,
				Build: BuildConfig{Runner: tc.runner,
					Defaults: &RecipeDefaults{Target: tc.defaultTarget, Mode: tc.defaultMode},
					Targets:  map[string]RecipeTarget{"macos": {}},
				},
			}}
			if tc.windowsRecipe {
				manifest.Manifest.Build.Targets["windows"] = RecipeTarget{}
			}
			wantTarget := tc.wantTarget
			context, err := ResolveBuildContext(manifest, BuildOptions{Target: tc.explicitTarget, Mode: tc.explicitMode})
			if err != nil {
				t.Fatal(err)
			}
			if context.Target != wantTarget || context.Mode != tc.wantMode {
				t.Fatalf("context = target %q mode %q, want %q / %q", context.Target, context.Mode, wantTarget, tc.wantMode)
			}
			if manifest.Manifest.Build.Defaults.Target != tc.defaultTarget {
				t.Fatal("host resolution changed the manifest default")
			}
		})
	}
}
