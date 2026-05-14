package main

import (
	"os"

	"observability-cascade-go-node/api"
)

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
