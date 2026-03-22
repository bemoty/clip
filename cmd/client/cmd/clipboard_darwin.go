//go:build darwin

package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

func readClipboard() ([]byte, string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return nil, "", fmt.Errorf("pbpaste: %w", err)
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}
	return out, "text/plain; charset=utf-8", nil
}

func writeClipboard(s string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
