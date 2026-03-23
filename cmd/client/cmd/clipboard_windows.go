//go:build windows

package cmd

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                      = syscall.NewLazyDLL("user32.dll")
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard           = user32.NewProc("OpenClipboard")
	procCloseClipboard          = user32.NewProc("CloseClipboard")
	procEmptyClipboard          = user32.NewProc("EmptyClipboard")
	procGetClipboardData        = user32.NewProc("GetClipboardData")
	procSetClipboardData        = user32.NewProc("SetClipboardData")
	procEnumClipboardFormats    = user32.NewProc("EnumClipboardFormats")
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	procGlobalFree              = kernel32.NewProc("GlobalFree")
	procGlobalLock              = kernel32.NewProc("GlobalLock")
	procGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
	procGlobalSize              = kernel32.NewProc("GlobalSize")
)

const (
	cfDIB         = 8      // device-independent bitmap (CF_DIB)
	cfHdrop       = 15     // file drop list (CF_HDROP)
	cfUnicodeText = 13     // UTF-16 text (CF_UNICODETEXT)
	gmemMoveable  = 0x0002 // SetClipboardData() requires a GMEM_MOVEABLE memory object
)

// cfPNG is registered at startup; PNG is not a built-in format so its ID is assigned at runtime.
var cfPNG = func() uint32 {
	name, _ := syscall.UTF16PtrFromString("PNG")
	r, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	return uint32(r)
}()

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

// listFormats enumerates all formats currently on the clipboard.
// The clipboard must be open before calling this.
func listFormats() []uint32 {
	var formats []uint32
	f, _, _ := procEnumClipboardFormats.Call(0)
	for f != 0 {
		formats = append(formats, uint32(f))
		f, _, _ = procEnumClipboardFormats.Call(f)
	}
	return formats
}

// pickFormat selects the best clipboard format from the available list using a fixed priority:
// CF_PNG > CF_DIB > CF_HDROP > CF_UNICODETEXT.
func pickFormat(formats []uint32) uint32 {
	for _, want := range []uint32{cfPNG, cfDIB, cfHdrop, cfUnicodeText} {
		for _, have := range formats {
			if have == want {
				return have
			}
		}
	}
	return 0
}

// readRawBytes locks a clipboard handle and returns a copy of its bytes.
func readRawBytes(h uintptr) ([]byte, error) {
	sz, _, _ := procGlobalSize.Call(h)
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		return nil, fmt.Errorf("GlobalLock: %w", err)
	}
	defer procGlobalUnlock.Call(h)
	data := make([]byte, sz)
	copy(data, (*[1 << 26]byte)(unsafe.Pointer(p))[:sz])
	return data, nil
}

// dibToBMP prepends a BITMAPFILEHEADER to raw CF_DIB bytes to produce a valid BMP file.
// CF_DIB starts with a BITMAPINFOHEADER but is missing the 14-byte file header that
// standard BMP readers expect.
func dibToBMP(dib []byte) ([]byte, error) {
	if len(dib) < 40 {
		return nil, fmt.Errorf("CF_DIB data too short")
	}
	biSize := binary.LittleEndian.Uint32(dib[0:4])
	biBitCount := binary.LittleEndian.Uint16(dib[14:16])
	biClrUsed := binary.LittleEndian.Uint32(dib[32:36])

	// For < 16-bit images the color table is mandatory; its size is 2^biBitCount entries
	// unless biClrUsed overrides it. For >= 16-bit images it is absent unless biClrUsed
	// explicitly requests one.
	var colorTableEntries uint32
	if biBitCount < 16 {
		if biClrUsed == 0 {
			colorTableEntries = 1 << biBitCount
		} else {
			colorTableEntries = biClrUsed
		}
	} else {
		colorTableEntries = biClrUsed
	}

	offBits := 14 + biSize + colorTableEntries*4
	fileSize := uint32(14 + len(dib))

	// BITMAPFILEHEADER layout (14 bytes):
	//   [0:2]   bfType     "BM"
	//   [2:6]   bfSize     total file size
	//   [6:10]  reserved   (zero)
	//   [10:14] bfOffBits  offset to pixel data
	header := make([]byte, 14)
	header[0], header[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(header[2:6], fileSize)
	binary.LittleEndian.PutUint32(header[10:14], offBits)
	return append(header, dib...), nil
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

	format := pickFormat(listFormats())
	if format == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}

	h, _, _ := procGetClipboardData.Call(uintptr(format))
	if h == 0 {
		return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
	}

	switch format {
	case cfPNG:
		data, err := readRawBytes(h)
		if err != nil {
			return nil, "", err
		}
		return data, "image/png", nil

	case cfDIB:
		raw, err := readRawBytes(h)
		if err != nil {
			return nil, "", err
		}
		data, err := dibToBMP(raw)
		if err != nil {
			return nil, "", err
		}
		return data, "image/bmp", nil

	case cfHdrop:
		// parseCFHDrop needs a raw pointer, not a byte slice, so lock manually
		// rather than going through readRawBytes.
		p, _, err := procGlobalLock.Call(h)
		if p == 0 {
			return nil, "", fmt.Errorf("GlobalLock: %w", err)
		}
		paths := parseCFHDrop(p)
		procGlobalUnlock.Call(h)
		data, err := resolveFile(paths)
		if err != nil {
			return nil, "", err
		}
		return data, "", nil

	case cfUnicodeText:
		p, _, err := procGlobalLock.Call(h)
		if p == 0 {
			return nil, "", fmt.Errorf("GlobalLock: %w", err)
		}
		defer procGlobalUnlock.Call(h)
		// Cast the raw pointer to a large uint16 slice; UTF16ToString stops at the null terminator.
		text := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
		return []byte(text), "text/plain; charset=utf-8", nil
	}

	return nil, "", fmt.Errorf("clipboard is empty, nothing to upload")
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
