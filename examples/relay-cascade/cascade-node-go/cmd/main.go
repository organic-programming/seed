package main

import (
	"os"

	"cascade-node-go/api"
)

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
