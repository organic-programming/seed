package server

import (
	holonserve "github.com/organic-programming/go-holons/pkg/serve"
	jamesloopsv1 "github.com/organic-programming/james-loops/gen/go/v1"
	"google.golang.org/grpc"
)

func ListenAndServe(listenURIs []string, reflect bool, handler jamesloopsv1.JamesLoopsServiceServer) error {
	if len(listenURIs) == 0 {
		listenURIs = []string{holonserve.ParseOptions(nil).ListenURI}
	}
	return holonserve.RunWithOptions(listenURIs[0], func(server *grpc.Server) {
		jamesloopsv1.RegisterJamesLoopsServiceServer(server, handler)
	}, reflect, listenURIs[1:]...)
}
