package main

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/bemoty/clip/internal/mimetype"
)

type Server struct {
	config Config
	store  Store
}

var langRegex = regexp.MustCompile(`^[a-z0-9]{1,20}$`)

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if len(s.config.AuthKeys) == 1 && s.config.AuthKeys[0] == "no-auth" {
		return true
	}

	var match int
	for _, authKey := range s.config.AuthKeys {
		expected := "Bearer " + authKey
		match |= subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected))
	}
	if match == 1 {
		return true
	}

	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
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

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ext := determineExtension(contentType, r.URL.Query().Get("lang"))

	id, err := s.store.SaveFile(r.Body, ext, r.ContentLength)
	if err != nil {
		if errors.Is(err, ErrStorageFull) {
			http.Error(w, "Storage limit exceeded", http.StatusInsufficientStorage)
			return
		}
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

func (s *Server) HandleServe(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")

	reader, filename, ok := s.store.OpenFile(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	defer func(reader io.ReadCloser) {
		_ = reader.Close()
	}(reader)

	expiry, metaErr := s.store.GetMeta(id)
	if metaErr != nil && !errors.Is(metaErr, os.ErrNotExist) {
		slog.Error("failed to read file metadata", "id", id, "error", metaErr)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if metaErr == nil {
		now := time.Now()
		if now.After(expiry) {
			if err := s.store.DeleteFile(id); err != nil {
				slog.Warn("failed to delete expired file", "id", id, "error", err)
			}
			http.Error(w, "Gone", http.StatusGone)
			return
		}
		w.Header().Set("Expires", expiry.UTC().Format(http.TimeFormat))
		w.Header().Set("Cache-Control", "max-age="+strconv.FormatInt(int64(expiry.Sub(now)/time.Second), 10))
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")

	head := make([]byte, 512)
	n, _ := reader.Read(head)
	head = head[:n]
	fullReader := io.MultiReader(bytes.NewReader(head), reader)

	detected := mimetype.Sniff(head, filepath.Ext(filename))
	fileType := detected.Type
	isText := detected.IsText
	if isText && strings.Contains(r.Header.Get("Accept"), "text/html") {
		content, err := io.ReadAll(fullReader)
		if err != nil {
			slog.Warn("failed to read file", "error", err)
			http.NotFound(w, r)
			return
		}
		if isRedirectableURL(string(content)) {
			http.Redirect(w, r, string(content), http.StatusFound)
			return
		}
		if err := renderText(w, filename, content, s.config.PasteStyle); err != nil {
			slog.Warn("failed to render text file", "error", err)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if _, copyErr := io.Copy(w, bytes.NewReader(content)); copyErr != nil {
				slog.Warn("failed to write content", "error", copyErr)
			}
		}
	} else {
		w.Header().Set("Content-Disposition", "inline; filename="+id+filepath.Ext(filename))
		if isText {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		} else if fileType != "" {
			w.Header().Set("Content-Type", fileType)
		}
		if _, err := io.Copy(w, fullReader); err != nil {
			slog.Warn("failed to write file content", "error", err)
		}
	}
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/")

	if err := s.store.DeleteFile(id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		slog.Error("failed to delete file", "id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	slog.Info("file deleted", "id", id)
}

func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func determineExtension(contentType, lang string) string {
	if strings.HasPrefix(contentType, "text/") && langRegex.MatchString(lang) {
		return "." + lang
	}

	if ext := mimetype.ExtensionFor(contentType); ext != "" {
		return ext
	}

	if lexer := lexers.MatchMimeType(contentType); lexer != nil {
		for _, filename := range lexer.Config().Filenames {
			if ext := filepath.Ext(filename); ext != "" {
				return ext
			}
		}
	}

	return ".bin"
}

func isRedirectableURL(input string) bool {
	u, err := url.Parse(input)
	if err != nil {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	return u.Host != ""
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
