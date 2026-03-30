package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
)

func Inspect(req *opv1.InspectRequest) (*opv1.InspectResponse, error) {
	if req == nil || strings.TrimSpace(req.GetTarget()) == "" {
		return nil, fmt.Errorf("target is required")
	}

	var (
		doc *inspectpkg.Document
		err error
	)
	target := req.GetTarget()
	if strings.Contains(target, ":") {
		doc, err = inspectRemote(target)
	} else {
		doc, err = inspectLocal(target)
	}
	if err != nil {
		return nil, err
	}

	return &opv1.InspectResponse{Document: inspectDocumentToProto(doc)}, nil
}

func inspectLocal(ref string) (*inspectpkg.Document, error) {
	catalog, err := inspectpkg.LoadLocal(ref)
	if err != nil {
		return nil, err
	}
	return catalog.Document, nil
}

func inspectRemote(address string) (*inspectpkg.Document, error) {
	conn, err := sdkconnect.Connect(address)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sdkconnect.Disconnect(conn) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := holonsv1.NewHolonMetaClient(conn)
	response, err := client.Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		return nil, err
	}
	return inspectpkg.FromDescribeResponse(response), nil
}
