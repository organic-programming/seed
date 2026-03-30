package main

import (
	"os"

	"github.com/organic-programming/grace-op/api"
)

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
