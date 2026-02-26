// hello — the simplest possible holon.
package main

import (
	"os"

	"github.com/organic-programming/examples/hello-world/pkg/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
