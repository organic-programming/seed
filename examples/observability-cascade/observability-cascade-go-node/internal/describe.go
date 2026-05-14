package internal

import (
	"observability-cascade-go-node/gen"

	"github.com/organic-programming/go-holons/pkg/describe"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}
