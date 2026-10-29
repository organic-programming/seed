// Package fileuri converts local discovery paths to interoperable file URIs.
package fileuri

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

func FromPath(path string) string {
	path = filepath.ToSlash(path)
	uri := url.URL{Scheme: "file", Path: path}
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(path, "//") {
			parts := strings.SplitN(strings.TrimPrefix(path, "//"), "/", 2)
			uri.Host = parts[0]
			uri.Path = "/"
			if len(parts) == 2 {
				uri.Path += parts[1]
			}
		} else if hasDrive(path) {
			uri.Path = "/" + path
		}
	}
	return uri.String()
}

func ToPath(raw string) (string, error) {
	uri, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if uri.Scheme != "file" {
		return "", fmt.Errorf("holon URL %q is not a local file target", raw)
	}
	path := uri.Path
	if runtime.GOOS == "windows" && hasDrive(uri.Host) && len(uri.Host) == 2 {
		// Read old discovery caches that serialized C:\\... as file://C:/...
		// without losing the drive or interpreting it as a UNC host.
		path = uri.Host + path
	} else if uri.Host != "" && uri.Host != "localhost" {
		path = "//" + uri.Host + path
	} else if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") && hasDrive(path[1:]) {
		path = path[1:]
	}
	if path == "" {
		return "", fmt.Errorf("holon URL %q has no path", raw)
	}
	return filepath.Clean(filepath.FromSlash(path)), nil
}

func hasDrive(path string) bool {
	return len(path) >= 2 && path[1] == ':' && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z'))
}
