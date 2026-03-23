//go:build windows

package cmd

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	procGlobalFree       = kernel32.NewProc("GlobalFree")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
)

const (
	cfHdrop       = 15     // HDROP (file URIs) clipboard format (CF_HDROP)
	cfUnicodeText = 13     // UTF-16 clipboard format (CF_UNICODETEXT)
	gmemMoveable  = 0x0002 // SetClipboardData() requires a GMEM_MOVEABLE memory object
)

// waitOpenClipboard retries OpenClipboard for up to one second; another app
// may hold the clipboard briefly while pasting.
func waitOpenClipboard() error {
	deadline := time.Now().Add(time.Second)
	var err error
	for time.Now().Before(deadline) {
		r, _, e := procOpenClipboard.Call(0)
		if r != 0 {
			return nil
		}
		err = e
		time.Sleep(time.Millisecond)
	}
	return fmt.Errorf("OpenClipboard: %w", err)
}

// parseCFHDrop reads the file paths out of a locked CF_HDROP memory block.
// The DROPFILES header is 20 bytes: pFiles uint32, pt [2]int32, fNC uint32, fWide uint32.
// fWide=1 means UTF-16 paths; fWide=0 means ANSI paths.
// The path list starts at pFiles and is double-null terminated.
func parseCFHDrop(p uintptr) []string {
	pFiles := *(*uint32)(unsafe.Pointer(p))
	fWide := *(*uint32)(unsafe.Pointer(p + 16))
	cur := p + uintptr(pFiles)

	var paths []string
	for {
		if fWide != 0 {
			s := syscall.UTF16ToString((*[1 << 15]uint16)(unsafe.Pointer(cur))[:])
			if s == "" {
				break
			}
			paths = append(paths, s)
			cur += uintptr(len(s)+1) * 2
		} else {
			b := (*[1 << 15]byte)(unsafe.Pointer(cur))[:]
			var n int
			// ANSI paths are null-terminated
			for n < len(b) && b[n] != 0 {
				n++
			}
			// Second null terminator marks end of list
			if n == 0 {
				break
			}
			paths = append(paths, string(b[:n]))
			cur += uintptr(n + 1)
		}
	}
	return paths
}

func readClipboard() ([]byte, string, error) {
	// OpenClipboard and CloseClipboard must run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := waitOpenClipboard(); err != nil {
		return nil, "", err
	}
	defer procCloseClipboard.Call()

	if c, _, _ := procGetClipboardData.Call(cfHdrop); c != 0 {
		p, _, err := procGlobalLock.Call(c)
		if p == 0 {
			return nil, "", fmt.Errorf("GlobalLock: %w", err)
		}
		paths := parseCFHDrop(p)
		procGlobalUnlock.Call(c)
		data, err := resolveFile(paths)
		if err != nil {
			return nil, "", err
		}
		return data, "", nil
	}

	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}

	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		return nil, "", fmt.Errorf("GlobalLock: %w", err)
	}
	defer procGlobalUnlock.Call(h)

	// Cast the raw pointer to a large uint16 slice; UTF16ToString stops at the null terminator.
	text := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
	return []byte(text), "text/plain; charset=utf-8", nil
}

func writeClipboard(s string) error {
	// OpenClipboard and CloseClipboard must run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u16, err := syscall.UTF16FromString(s)
	if err != nil {
		return err
	}
	size := uintptr(len(u16)) * unsafe.Sizeof(u16[0])

	if err := waitOpenClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	r, _, err := procEmptyClipboard.Call()
	if r == 0 {
		return fmt.Errorf("EmptyClipboard: %w", err)
	}

	h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("GlobalAlloc: %w", err)
	}
	defer func() {
		if h != 0 {
			procGlobalFree.Call(h)
		}
	}()

	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		return fmt.Errorf("GlobalLock: %w", err)
	}
	dst := (*[1 << 20]uint16)(unsafe.Pointer(p))[:len(u16)]
	copy(dst, u16)

	r, _, err = procGlobalUnlock.Call(h)
	if r == 0 {
		if err.(syscall.Errno) != 0 {
			return fmt.Errorf("GlobalUnlock: %w", err)
		}
	}

	r, _, err = procSetClipboardData.Call(cfUnicodeText, h)
	if r == 0 {
		return fmt.Errorf("SetClipboardData: %w", err)
	}
	h = 0 // OS now owns the handle; suppress deferred GlobalFree
	return nil
}
