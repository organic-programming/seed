package main

import (
	"os"

	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/grace-op/gen"
	"github.com/organic-programming/grace-op/internal/cli"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}

func main() {
	os.Exit(cli.Execute())
}
