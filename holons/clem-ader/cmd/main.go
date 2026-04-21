package main

import (
	"os"

	"github.com/organic-programming/clem-ader/api"
	"github.com/organic-programming/clem-ader/gen"
	"github.com/organic-programming/go-holons/pkg/describe"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}

func main() {
	os.Exit(api.RunCLI(os.Args[1:]))
}
