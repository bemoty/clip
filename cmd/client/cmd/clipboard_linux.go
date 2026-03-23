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

func listMIMETypes() ([]string, error) {
	var (
		out []byte
		err error
	)
	if isWayland() {
		out, err = exec.Command("wl-paste", "--list-types").Output()
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return nil, fmt.Errorf("wl-paste not found; install wl-clipboard")
			}
			// wl-paste exits non-zero when the clipboard is empty
			return nil, fmt.Errorf("clipboard is empty, nothing to upload")
		}
	} else {
		out, err = exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return nil, fmt.Errorf("xclip not found; install xclip")
			}
			return nil, fmt.Errorf("clipboard is empty, nothing to upload")
		}
	}

	var types []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		t := strings.TrimSpace(line)
		// Filter X11 atoms that are not MIME types (e.g., TARGETS, TIMESTAMP, UTF8_STRING)
		if t != "" && strings.Contains(t, "/") {
			types = append(types, t)
		}
	}
	return types, nil
}

func readBytes(mimeType string) ([]byte, error) {
	var cmd *exec.Cmd
	if isWayland() {
		args := []string{"-t", mimeType}
		if strings.HasPrefix(mimeType, "text/") {
			// wl-paste appends a newline to text content; suppress it
			args = append([]string{"--no-newline"}, args...)
		}
		cmd = exec.Command("wl-paste", args...)
	} else {
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", mimeType, "-o")
	}
	return cmd.Output()
}

// pickMIMEType selects the best MIME type from the available list using a fixed priority:
// image/png > other image/* > text/plain;charset=utf-8 > text/plain > other text/* > first available
func pickMIMEType(types []string) string {
	for _, t := range types {
		if t == "image/png" {
			return t
		}
	}
	for _, t := range types {
		if strings.HasPrefix(t, "image/") {
			return t
		}
	}
	for _, t := range types {
		if t == "text/plain;charset=utf-8" || t == "text/plain; charset=utf-8" {
			return t
		}
	}
	for _, t := range types {
		if t == "text/plain" {
			return t
		}
	}
	for _, t := range types {
		if strings.HasPrefix(t, "text/") {
			return t
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return ""
}

func readClipboard() ([]byte, string, error) {
	types, err := listMIMETypes()
	if err != nil {
		return nil, "", err
	}
	if len(types) == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}

	mimeType := pickMIMEType(types)
	data, err := readBytes(mimeType)
	if err != nil {
		return nil, "", fmt.Errorf("clipboard read: %w", err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}
	if mimeType == "text/uri-list" {
		var uris []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				uris = append(uris, line)
			}
		}
		data, err = resolveFile(uris)
		if err != nil {
			return nil, "", err
		}
		return data, "", nil
	}
	return data, mimeType, nil
}

func writeClipboard(s string) error {
	var cmd *exec.Cmd
	var pkg string
	if isWayland() {
		cmd = exec.Command("wl-copy")
		pkg = "wl-clipboard"
	} else {
		cmd = exec.Command("xclip", "-selection", "clipboard")
		pkg = "xclip"
	}
	cmd.Stdin = strings.NewReader(s)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("%s not found; install %s", cmd.Args[0], pkg)
		}
		return fmt.Errorf("clipboard write: %w", err)
	}
	return nil
}
