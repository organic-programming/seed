package internal

import (
	"cascade-node-go/gen"

	"github.com/organic-programming/go-holons/pkg/describe"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}
