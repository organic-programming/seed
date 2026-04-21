package runpolicy

import (
	"fmt"
	"strings"
)

const DefaultRunListenURI = "tcp://127.0.0.1:0"

func NormalizeRunListenURI(raw string, explicit bool) (string, error) {
	listenURI := strings.TrimSpace(raw)
	if listenURI == "" {
		return DefaultRunListenURI, nil
	}
	if explicit && IsStdioURI(listenURI) {
		return "", fmt.Errorf("--listen stdio:// is not supported for op run; use stdio://<holon> <method> for stdio RPC")
	}
	return listenURI, nil
}

func IsStdioURI(uri string) bool {
	switch strings.ToLower(strings.TrimSpace(uri)) {
	case "stdio://", "stdio":
		return true
	default:
		return false
	}
}
