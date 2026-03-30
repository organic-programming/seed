package api

import (
	"fmt"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/holons"
)

func Check(req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return runLifecycle(holons.OperationCheck, req)
}

func Build(req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return runLifecycle(holons.OperationBuild, req)
}

func Test(req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return runLifecycle(holons.OperationTest, req)
}

func Clean(req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	return runLifecycle(holons.OperationClean, req)
}

func runLifecycle(operation holons.Operation, req *opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) {
	target := "."
	if req != nil && req.GetTarget() != "" {
		target = req.GetTarget()
	}

	report, err := holons.ExecuteLifecycle(operation, target, buildOptionsFromProto(nilIfNoBuild(req)))
	return &opv1.LifecycleResponse{Report: lifecycleReportToProto(report)}, err
}

func nilIfNoBuild(req *opv1.LifecycleRequest) *opv1.BuildOptions {
	if req == nil {
		return nil
	}
	return req.GetBuild()
}

func Install(req *opv1.InstallRequest) (*opv1.InstallResponse, error) {
	target := "."
	if req != nil && req.GetTarget() != "" {
		target = req.GetTarget()
	}

	report, err := holons.Install(target, holons.InstallOptions{
		Build:            req != nil && req.GetBuild(),
		LinkApplications: req != nil && req.GetLinkApplications(),
	})
	return &opv1.InstallResponse{Report: installReportToProto(report)}, err
}

func Uninstall(req *opv1.UninstallRequest) (*opv1.InstallResponse, error) {
	if req == nil || req.GetTarget() == "" {
		return nil, fmt.Errorf("target is required")
	}
	report, err := holons.UninstallWithOptions(req.GetTarget(), holons.InstallOptions{})
	return &opv1.InstallResponse{Report: installReportToProto(report)}, err
}
