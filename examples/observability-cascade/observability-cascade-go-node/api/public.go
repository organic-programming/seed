package api

import (
	"context"

	pb "observability-cascade-go-node/gen/go/relay/v1"
	"observability-cascade-go-node/internal"
)

// Tick emits one cascade tick through the in-process implementation.
func Tick(req *pb.TickRequest) (*pb.TickResponse, error) {
	return (&internal.Server{}).Tick(context.Background(), req)
}
