//go:build ignore

package main

import (
	"fmt"
	"os"

	seedtoolchain "github.com/organic-programming/seed-github-scripts/seed_toolchain"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: seed_toolchain <command> <repo-root> [...]")
		os.Exit(2)
	}
	command := os.Args[1]
	repoRoot := os.Args[2]
	seed, err := seedtoolchain.Load(repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch command {
	case "protoc-version":
		fmt.Println(seedtoolchain.ProtocVersion(seed))
	case "seed-release":
		fmt.Println(seedtoolchain.SeedRelease(seed))
	case "cpp-protobuf-tag":
		fmt.Println(seedtoolchain.CPPProtobufTag(seed))
	case "plugin-version":
		if len(os.Args) != 5 {
			fmt.Fprintln(os.Stderr, "usage: seed_toolchain plugin-version <repo-root> <lang> <plugin>")
			os.Exit(2)
		}
		fmt.Println(seedtoolchain.PluginVersion(seed, os.Args[3], os.Args[4]))
	case "plugin-sha256":
		if len(os.Args) != 6 {
			fmt.Fprintln(os.Stderr, "usage: seed_toolchain plugin-sha256 <repo-root> <lang> <plugin> <target>")
			os.Exit(2)
		}
		fmt.Println(seedtoolchain.PluginSHA256(seed, os.Args[3], os.Args[4], os.Args[5]))
	case "manifest-json":
		if len(os.Args) != 5 {
			fmt.Fprintln(os.Stderr, "usage: seed_toolchain manifest-json <repo-root> <lang> <target>")
			os.Exit(2)
		}
		data, err := seedtoolchain.ManifestJSON(seed, os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(string(data))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		os.Exit(1)
	}
}
