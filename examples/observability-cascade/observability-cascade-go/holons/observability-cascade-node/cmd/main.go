package main

import (
	"os"

	"observability-cascade-node-go/api"
)

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
