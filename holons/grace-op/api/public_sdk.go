package api

import (
	"context"
	"fmt"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/sdkprebuilts"
)

func InstallSdkPrebuilt(req *opv1.InstallSdkPrebuiltRequest) (*opv1.SdkPrebuiltResponse, error) {
	return installSdkPrebuiltContext(context.Background(), req)
}

func installSdkPrebuiltContext(ctx context.Context, req *opv1.InstallSdkPrebuiltRequest) (*opv1.SdkPrebuiltResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("sdk prebuilt install request is required")
	}
	prebuilt, notes, err := sdkprebuilts.Install(ctx, sdkprebuilts.InstallOptions{
		Lang:    req.GetLang(),
		Target:  req.GetTarget(),
		Version: req.GetVersion(),
		Source:  req.GetSource(),
	})
	if err != nil {
		return nil, err
	}
	return &opv1.SdkPrebuiltResponse{
		Prebuilt: sdkPrebuiltToProto(prebuilt),
		Notes:    notes,
	}, nil
}

func ListSdkPrebuilts(req *opv1.ListSdkPrebuiltsRequest) (*opv1.ListSdkPrebuiltsResponse, error) {
	lang := ""
	installed := true
	available := false
	if req != nil {
		lang = req.GetLang()
		installed = req.GetInstalled()
		available = req.GetAvailable()
		if !installed && !available {
			installed = true
		}
	}

	resp := &opv1.ListSdkPrebuiltsResponse{}
	if installed {
		entries, err := sdkprebuilts.ListInstalled(lang)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			resp.Entries = append(resp.Entries, sdkPrebuiltToProto(entry))
		}
	}
	if available {
		entries, notes, err := sdkprebuilts.ListAvailable(lang)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			resp.Entries = append(resp.Entries, sdkPrebuiltToProto(entry))
		}
		resp.Notes = append(resp.Notes, notes...)
	}
	return resp, nil
}

func UninstallSdkPrebuilt(req *opv1.UninstallSdkPrebuiltRequest) (*opv1.SdkPrebuiltResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("sdk prebuilt uninstall request is required")
	}
	prebuilt, err := sdkprebuilts.Uninstall(sdkprebuilts.QueryOptions{
		Lang:    req.GetLang(),
		Target:  req.GetTarget(),
		Version: req.GetVersion(),
	})
	if err != nil {
		return nil, err
	}
	return &opv1.SdkPrebuiltResponse{Prebuilt: sdkPrebuiltToProto(prebuilt)}, nil
}

func VerifySdkPrebuilt(req *opv1.VerifySdkPrebuiltRequest) (*opv1.SdkPrebuiltResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("sdk prebuilt verify request is required")
	}
	prebuilt, verified, err := sdkprebuilts.Verify(sdkprebuilts.QueryOptions{
		Lang:    req.GetLang(),
		Target:  req.GetTarget(),
		Version: req.GetVersion(),
	})
	if err != nil {
		return nil, err
	}
	return &opv1.SdkPrebuiltResponse{
		Prebuilt: sdkPrebuiltToProto(prebuilt),
		Verified: verified,
	}, nil
}

func LocateSdkPrebuilt(req *opv1.LocateSdkPrebuiltRequest) (*opv1.SdkPrebuiltResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("sdk prebuilt locate request is required")
	}
	prebuilt, err := sdkprebuilts.Locate(sdkprebuilts.QueryOptions{
		Lang:    req.GetLang(),
		Target:  req.GetTarget(),
		Version: req.GetVersion(),
	})
	if err != nil {
		return nil, err
	}
	return &opv1.SdkPrebuiltResponse{Prebuilt: sdkPrebuiltToProto(prebuilt)}, nil
}

func sdkPrebuiltToProto(prebuilt sdkprebuilts.Prebuilt) *opv1.SdkPrebuilt {
	return &opv1.SdkPrebuilt{
		Lang:          prebuilt.Lang,
		Version:       prebuilt.Version,
		Target:        prebuilt.Target,
		Path:          prebuilt.Path,
		Source:        prebuilt.Source,
		ArchiveSha256: prebuilt.ArchiveSHA256,
		TreeSha256:    prebuilt.TreeSHA256,
		Installed:     prebuilt.Installed,
	}
}
