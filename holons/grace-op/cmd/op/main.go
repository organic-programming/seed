package main

import (
	"os"

	"github.com/organic-programming/grace-op/api"
	"github.com/organic-programming/grace-op/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], api.VersionString()))
}
