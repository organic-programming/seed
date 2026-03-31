package main

import (
	"os"

	"github.com/organic-programming/grace-op/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
