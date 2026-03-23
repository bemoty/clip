//go:build linux

package cmd

import "testing"

func TestPickMIMEType(t *testing.T) {
	tests := []struct {
		name  string
		types []string
		want  string
	}{
		{
			name:  "prefers image/png over other images",
			types: []string{"image/bmp", "image/png", "image/jpeg"},
			want:  "image/png",
		},
		{
			name:  "picks first image when no png",
			types: []string{"image/jpeg", "image/webp"},
			want:  "image/jpeg",
		},
		{
			name:  "prefers image over text",
			types: []string{"text/plain", "image/png"},
			want:  "image/png",
		},
		{
			name:  "prefers utf-8 text over plain",
			types: []string{"text/plain", "text/plain;charset=utf-8"},
			want:  "text/plain;charset=utf-8",
		},
		{
			name:  "accepts utf-8 text with space after semicolon",
			types: []string{"text/plain; charset=utf-8"},
			want:  "text/plain; charset=utf-8",
		},
		{
			name:  "prefers text/plain over other text subtypes",
			types: []string{"text/html", "text/plain"},
			want:  "text/plain",
		},
		{
			name:  "falls back to first text subtype",
			types: []string{"text/html", "text/rtf"},
			want:  "text/html",
		},
		{
			name:  "falls back to first available for non-text non-image",
			types: []string{"application/pdf"},
			want:  "application/pdf",
		},
		{
			name:  "empty list returns empty string",
			types: []string{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickMIMEType(tt.types)
			if got != tt.want {
				t.Errorf("pickMIMEType(%v) = %q, want %q", tt.types, got, tt.want)
			}
		})
	}
}
