package main

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

type Server struct {
	config Config
	store  *DiskStore
}

var langRegex = regexp.MustCompile(`^[a-z0-9]{1,20}$`)

const maxTTL = 365 * 24 * time.Hour

func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	expectedAuthHeader := "Bearer " + s.config.AuthKey
	if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expectedAuthHeader)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ttl := s.config.DefaultTTL
	if ttlStr := r.URL.Query().Get("ttl"); ttlStr != "" {
		var err error
		ttl, err = parseTTL(ttlStr)
		if err != nil {
			http.Error(w, "Invalid TTL", http.StatusBadRequest)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxFileMB<<20)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Warn("failed to close request body", "error", err)
		}
	}(r.Body)

	head := make([]byte, 512)
	n, err := r.Body.Read(head)
	if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
		slog.Error("failed to read request header", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(head[:n])
	}
	ext := determineExtension(contentType, r.URL.Query().Get("lang"))

	fullBody := io.MultiReader(bytes.NewReader(head[:n]), r.Body)
	id, err := s.store.SaveFile(fullBody, ext)
	if err != nil {
		slog.Error("failed to save file", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if ttl > 0 {
		if err := s.store.WriteMeta(id, ttl); err != nil {
			slog.Warn("failed to write ttl metadata", "id", id, "error", err)
		}
	}

	fullURL := s.config.BaseURL + "/" + id
	if _, err := w.Write([]byte(fullURL)); err != nil {
		slog.Warn("failed to write response to client", "error", err)
	}

	slog.Info("file uploaded", "id", id, "type", contentType, "url", fullURL)
}

func determineExtension(contentType, lang string) string {
	if strings.HasPrefix(contentType, "text/") && langRegex.MatchString(lang) {
		return "." + lang
	}

	exts, err := mime.ExtensionsByType(contentType)
	if len(exts) == 0 || err != nil {
		return ".bin"
	}
	return exts[0]
}

func (s *Server) HandleServe(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")

	path, ok := s.store.GetFile(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	absBaseDir, err := filepath.Abs(s.store.BaseDir)
	absPath, err2 := filepath.Abs(path)
	if err != nil || err2 != nil || !strings.HasPrefix(absPath, absBaseDir+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}

	expiry, metaErr := s.store.GetMeta(id)
	if metaErr != nil && !errors.Is(metaErr, os.ErrNotExist) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if metaErr == nil && time.Now().After(expiry) {
		if err := s.store.DeleteFile(id); err != nil {
			slog.Warn("failed to delete expired file", "id", id, "error", err)
		}
		if err := s.store.DeleteMeta(id); err != nil {
			slog.Warn("failed to delete expired file metadata", "id", id, "error", err)
		}
		http.Error(w, "Gone", http.StatusGone)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")

	fileType := mime.TypeByExtension(filepath.Ext(path))
	if strings.HasPrefix(fileType, "text/") && strings.Contains(r.Header.Get("Accept"), "text/html") {
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("failed to read file", "error", err)
			http.NotFound(w, r)
			return
		}
		if err := renderText(w, path, content, s.config.PasteStyle); err != nil {
			slog.Warn("failed to render text file", "error", err)
			http.ServeFile(w, r, path)
		}
	} else {
		w.Header().Set("Content-Disposition", "inline; filename="+id+filepath.Ext(path))
		http.ServeFile(w, r, path)
	}
}

func parseTTL(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid ttl %q", s)
		}
		return validateTTL(time.Duration(n) * 24 * time.Hour)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return validateTTL(d)
}

func validateTTL(d time.Duration) (time.Duration, error) {
	if d > maxTTL {
		return 0, errors.New("ttl too large")
	}
	if d <= 0 {
		return 0, errors.New("ttl cannot be negative or zero")
	}
	return d, nil
}

func renderText(w http.ResponseWriter, path string, content []byte, style string) error {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(string(content))
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	it, err := lexer.Tokenise(nil, string(content))
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	formatter := html.New(html.Standalone(true), html.WithClasses(true))
	return formatter.Format(w, styles.Get(style), it)
}
