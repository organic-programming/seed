package internal

import (
	"matt-calculator-go/gen"

	"github.com/organic-programming/go-holons/pkg/describe"
)

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}
