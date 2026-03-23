//go:build windows

package cmd

import "testing"

func TestPickFormat(t *testing.T) {
	tests := []struct {
		name    string
		formats []uint32
		want    uint32
	}{
		{
			name:    "prefers CF_PNG over all others",
			formats: []uint32{cfUnicodeText, cfDIB, cfHdrop, cfPNG},
			want:    cfPNG,
		},
		{
			name:    "prefers CF_DIB over CF_HDROP and text",
			formats: []uint32{cfUnicodeText, cfHdrop, cfDIB},
			want:    cfDIB,
		},
		{
			name:    "prefers CF_HDROP over text",
			formats: []uint32{cfUnicodeText, cfHdrop},
			want:    cfHdrop,
		},
		{
			name:    "falls back to CF_UNICODETEXT",
			formats: []uint32{cfUnicodeText},
			want:    cfUnicodeText,
		},
		{
			name:    "returns 0 for unknown formats",
			formats: []uint32{42, 99},
			want:    0,
		},
		{
			name:    "empty list returns 0",
			formats: []uint32{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickFormat(tt.formats)
			if got != tt.want {
				t.Errorf("pickFormat(%v) = %d, want %d", tt.formats, got, tt.want)
			}
		})
	}
}
