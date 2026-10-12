package main

import (
	"fmt"
	"os"

	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/processowner"
	"github.com/organic-programming/grace-op/gen"
	"github.com/organic-programming/grace-op/internal/cli"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}

func main() {
	if err := processowner.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(cli.Execute())
}
