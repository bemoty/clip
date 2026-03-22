//go:build linux

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func isWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland"
}

func clipTool() (readArgs []string, writeArgs []string, pkg string) {
	if isWayland() {
		return []string{"wl-paste", "--no-newline"}, []string{"wl-copy"}, "wl-clipboard"
	}
	return []string{"xclip", "-selection", "clipboard", "-o"}, []string{"xclip", "-selection", "clipboard"}, "xclip"
}

func readClipboard() ([]byte, string, error) {
	readArgs, _, pkg := clipTool()
	out, err := exec.Command(readArgs[0], readArgs[1:]...).Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, "", fmt.Errorf("%s not found; install %s", readArgs[0], pkg)
		}
		return nil, "", fmt.Errorf("clipboard read: %w", err)
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}
	return out, "text/plain; charset=utf-8", nil
}

func writeClipboard(s string) error {
	_, writeArgs, pkg := clipTool()
	cmd := exec.Command(writeArgs[0], writeArgs[1:]...)
	cmd.Stdin = strings.NewReader(s)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("%s not found; install %s", writeArgs[0], pkg)
		}
		return fmt.Errorf("clipboard write: %w", err)
	}
	return nil
}
