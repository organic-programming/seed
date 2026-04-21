package describe

import (
	"errors"
	"fmt"
	"sync"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// ErrNoIncodeDescription indicates that op build has not generated and wired
// a static DescribeResponse for the running holon.
var ErrNoIncodeDescription = errors.New("no Incode Description registered — run op build")

var (
	staticMu       sync.RWMutex
	staticResponse *holonsv1.DescribeResponse
)

// UseStaticResponse registers the build-generated DescribeResponse used by
// Register at runtime. Passing nil clears the current registration.
func UseStaticResponse(response *holonsv1.DescribeResponse) {
	staticMu.Lock()
	defer staticMu.Unlock()
	staticResponse = cloneDescribeResponse(response)
}

// StaticResponse returns the build-generated DescribeResponse currently
// registered for runtime use. The returned value is a defensive clone and may
// be mutated by the caller.
func StaticResponse() *holonsv1.DescribeResponse {
	return registeredStaticResponse()
}

// Register installs the build-generated HolonMeta implementation on the
// supplied gRPC server.
func Register(s *grpc.Server) error {
	if s == nil {
		return fmt.Errorf("grpc server is required")
	}

	response := registeredStaticResponse()
	if response == nil {
		return ErrNoIncodeDescription
	}

	holonsv1.RegisterHolonMetaServer(s, &metaServer{response: response})
	return nil
}

func registeredStaticResponse() *holonsv1.DescribeResponse {
	staticMu.RLock()
	defer staticMu.RUnlock()
	return cloneDescribeResponse(staticResponse)
}

func cloneDescribeResponse(response *holonsv1.DescribeResponse) *holonsv1.DescribeResponse {
	if response == nil {
		return nil
	}
	return proto.Clone(response).(*holonsv1.DescribeResponse)
}
