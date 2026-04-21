package internal

import (
	"gabriel-greeting-go/gen"

	"github.com/organic-programming/go-holons/pkg/describe"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}
