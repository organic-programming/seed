//go:build e2e

package invoke_test

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = flag.Set("test.timeout", "90m")
	os.Exit(m.Run())
}
