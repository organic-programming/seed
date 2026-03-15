package main

import (
	"os"

	"gabriel-greeting-go/api"
)

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
