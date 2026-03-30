package api

import (
	openv "github.com/organic-programming/grace-op/internal/env"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func Env(req *opv1.EnvRequest) (*opv1.EnvResponse, error) {
	initDirs := req != nil && req.GetInit()
	shell := req != nil && req.GetShell()

	if initDirs {
		if err := openv.Init(); err != nil {
			return nil, err
		}
	}

	response := &opv1.EnvResponse{
		Oppath:      openv.OPPATH(),
		Opbin:       openv.OPBIN(),
		Root:        openv.Root(),
		Initialized: initDirs,
		CacheDir:    openv.CacheDir(),
	}
	if shell {
		response.Shell = openv.ShellSnippet()
	}
	return response, nil
}
