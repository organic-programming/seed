package main

import (
	"os"

	"matt-calculator-go/api"
)

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
