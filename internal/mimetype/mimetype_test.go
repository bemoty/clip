package mimetype

import "testing"

func TestSniff(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		ext  string
		want string
	}{
		{"markdown by extension", []byte("# hi\n"), ".md", "text/markdown; charset=utf-8"},
		{"markdown with no extension still sniffs as text", []byte("# hi\n"), "", "text/plain; charset=utf-8"},
		{"server placeholder extension is ignored, bytes win", []byte("# hi\n"), ".bin", "text/plain; charset=utf-8"},
		{"no extension, no filename info, still sniffs as text", []byte("plain text content"), "", "text/plain; charset=utf-8"},

		{"pdf by extension", []byte("%PDF-1.4\n%..."), ".pdf", "application/pdf"},
		{"pdf by sniffing alone", []byte("%PDF-1.4\n%..."), "", "application/pdf"},
		{"zip by sniffing alone", []byte("PK\x03\x04rest of zip"), "", "application/zip"},
		{"gzip by sniffing alone", []byte("\x1f\x8b\x08\x00rest"), "", "application/x-gzip"},
		{"rar by extension", rarHeader, ".rar", "application/x-rar-compressed"},
		{"rar by sniffing alone", rarHeader, "", "application/x-rar-compressed"},
		{"midi by sniffing alone", midiHeader, "", "audio/midi"},
		{"avi by sniffing alone", aviHeader, "", "video/avi"},
		{"ttf by sniffing alone", []byte("\x00\x01\x00\x00rest"), "", "font/ttf"},
		{"woff by sniffing alone", []byte("wOFFrest"), "", "font/woff"},

		{"unrecognized binary falls back to octet-stream", []byte{0x01, 0x02, 0x03, 0x04}, "", Unknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sniff(tc.head, tc.ext)
			if got.Type != tc.want {
				t.Fatalf("Sniff(%q, %q).Type = %q, want %q", tc.head, tc.ext, got.Type, tc.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		head []byte
	}{
		{"markdown", []byte("# hi\n")},
		{"pdf", []byte("%PDF-1.4\n%...")},
		{"zip", []byte("PK\x03\x04rest of zip")},
		{"rar", rarHeader},
		{"midi", midiHeader},
		{"avi", aviHeader},
		{"ttf", []byte("\x00\x01\x00\x00rest")},
		{"woff", []byte("wOFFrest")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uploaded := Sniff(tc.head, "")

			ext := ExtensionFor(uploaded.Type)
			if ext == "" {
				t.Fatalf("ExtensionFor(%q) = \"\", want a known extension", uploaded.Type)
			}

			served := Sniff(tc.head, ext)
			if served.Type != uploaded.Type {
				t.Fatalf("round trip through extension %q changed type: uploaded as %q, served as %q", ext, uploaded.Type, served.Type)
			}
		})
	}
}

var (
	rarHeader  = []byte("Rar!\x1a\x07\x00rest")
	midiHeader = []byte("MThd\x00\x00\x00\x06")
	aviHeader  = []byte("RIFF\x00\x00\x00\x00AVI ")
)
