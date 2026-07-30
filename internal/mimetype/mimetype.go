package mimetype

import (
	"mime"
	"net/http"
	"strings"
)

const Unknown = "application/octet-stream"

type Result struct {
	Type   string
	IsText bool
}

var registry = []struct{ ext, mimeType string }{
	{".md", "text/markdown"},

	{".ico", "image/x-icon"},
	{".wav", "audio/wave"},
	{".ogg", "application/ogg"},
	{".aiff", "audio/aiff"},
	{".aif", "audio/aiff"},
	{".mid", "audio/midi"},
	{".midi", "audio/midi"},
	{".avi", "video/avi"},
	{".rar", "application/x-rar-compressed"},
	{".eot", "application/vnd.ms-fontobject"},
	{".ttf", "font/ttf"},
	{".otf", "font/otf"},
	{".ttc", "font/collection"},
	{".woff", "font/woff"},
	{".woff2", "font/woff2"},
}

func init() {
	for _, e := range registry {
		if err := mime.AddExtensionType(e.ext, e.mimeType); err != nil {
			panic(err)
		}
	}
}

func Sniff(head []byte, ext string) Result {
	var t string
	if ext != "" && ext != ".bin" {
		t = mime.TypeByExtension(ext)
	}
	if t == "" {
		t = http.DetectContentType(head)
	}
	return Result{Type: t, IsText: strings.HasPrefix(t, "text/")}
}

func ExtensionFor(contentType string) string {
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}
