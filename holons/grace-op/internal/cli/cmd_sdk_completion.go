package cli

import (
	"sort"
	"strings"

	"github.com/organic-programming/grace-op/internal/sdkprebuilts"
	"github.com/spf13/cobra"
)

// completeInstalledSdkLangs lists langs that have a prebuilt installed
// under $OPPATH/sdk/. Used for op sdk uninstall/verify/path positional.
func completeInstalledSdkLangs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := sdkprebuilts.ListInstalled("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return uniqueLangsMatching(entries, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeAvailableSdkLangs lists langs available as published releases.
// Used for op sdk install positional. Falls back silently to nothing if
// the network call fails — completion must never block the shell.
func completeAvailableSdkLangs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, _, err := sdkprebuilts.ListAvailable("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return uniqueLangsMatching(entries, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeCompilableSdkLangs lists langs whose build script + submodules
// + binaries are present on this checkout. Used for op sdk build positional.
func completeCompilableSdkLangs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, _, err := sdkprebuilts.ListCompilable("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(entries))
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if len(entry.Blockers) > 0 {
			continue
		}
		if !strings.HasPrefix(entry.Lang, toComplete) {
			continue
		}
		if _, ok := seen[entry.Lang]; ok {
			continue
		}
		seen[entry.Lang] = struct{}{}
		out = append(out, entry.Lang)
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeAllSdkLangs returns the union of installed, available, and
// compilable lang names. Used for the --lang filter on op sdk list.
func completeAllSdkLangs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	seen := make(map[string]struct{})
	collect := func(entries []sdkprebuilts.Prebuilt) {
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Lang, toComplete) {
				continue
			}
			seen[entry.Lang] = struct{}{}
		}
	}
	if entries, err := sdkprebuilts.ListInstalled(""); err == nil {
		collect(entries)
	}
	if entries, _, err := sdkprebuilts.ListAvailable(""); err == nil {
		collect(entries)
	}
	if entries, _, err := sdkprebuilts.ListCompilable(""); err == nil {
		collect(entries)
	}
	out := make([]string, 0, len(seen))
	for lang := range seen {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeAllowedSdkTargets returns the static list of supported target
// triplets. Used for the --target flag on every op sdk verb.
func completeAllowedSdkTargets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	targets := sdkprebuilts.AllowedTargets()
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		if strings.HasPrefix(t, toComplete) {
			out = append(out, t)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeSdkVersionsForLang returns the default version for the lang
// already typed at args[0], plus any installed/available versions found
// in the local inventory. Used for the --version flag.
func completeSdkVersionsForLang(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	lang := strings.TrimSpace(args[0])
	if lang == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	versions := make(map[string]struct{})
	if def, ok := sdkprebuilts.DefaultVersion(lang); ok {
		versions[def] = struct{}{}
	}
	if entries, err := sdkprebuilts.ListInstalled(lang); err == nil {
		for _, entry := range entries {
			versions[entry.Version] = struct{}{}
		}
	}
	if entries, _, err := sdkprebuilts.ListAvailable(lang); err == nil {
		for _, entry := range entries {
			versions[entry.Version] = struct{}{}
		}
	}
	out := make([]string, 0, len(versions))
	for v := range versions {
		if strings.HasPrefix(v, toComplete) {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}

// uniqueLangsMatching returns sorted unique Lang values from entries
// where Lang has the toComplete prefix.
func uniqueLangsMatching(entries []sdkprebuilts.Prebuilt, toComplete string) []string {
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Lang, toComplete) {
			continue
		}
		seen[entry.Lang] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for lang := range seen {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}
