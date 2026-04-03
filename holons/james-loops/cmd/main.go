package main

import (
	"os"

	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/james-loops/api"
	"github.com/organic-programming/james-loops/gen"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
