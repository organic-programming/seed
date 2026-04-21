//go:build e2e

package build_test

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = flag.Set("test.timeout", "30m")
	os.Exit(m.Run())
}
