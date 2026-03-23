package cmd

import (
	"fmt"
	"os"
	"strings"
)

func resolveFile(uris []string) ([]byte, error) {
	switch {
	case len(uris) == 0:
		return nil, fmt.Errorf("no file specified, nothing to upload")
	case len(uris) > 1:
		return nil, fmt.Errorf("multiple files copied; clip only supports one at a time")
	default:
		path := strings.TrimSpace(strings.TrimPrefix(uris[0], "file://"))
		return os.ReadFile(path)
	}
}
