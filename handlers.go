package main

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
)

type Server struct {
	config Config
	store  *DiskStore
}

func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	expectedAuthHeader := "Bearer " + s.config.AuthKey
	if authHeader != expectedAuthHeader {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
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

	contentType := http.DetectContentType(head[:n])
	if !strings.HasPrefix(contentType, "image/") &&
		!strings.HasPrefix(contentType, "video/") &&
		!strings.HasPrefix(contentType, "audio/") {
		http.Error(w, "Unsupported Media Type "+contentType, http.StatusUnsupportedMediaType)
		return
	}

	exts, err := mime.ExtensionsByType(contentType)
	if len(exts) == 0 || err != nil {
		http.Error(w, "Unknown Media Type "+contentType, http.StatusInternalServerError)
		return
	}
	ext := exts[0]

	fullBody := io.MultiReader(bytes.NewReader(head[:n]), r.Body)
	id, err := s.store.SaveFile(fullBody, ext)
	if err != nil {
		slog.Error("failed to save file", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fullURL := "https://" + r.Host + "/" + id
	if _, err := w.Write([]byte(fullURL)); err != nil {
		slog.Warn("failed to write response to client", "error", err)
	}

	slog.Info("file uploaded", "id", id, "type", contentType, "url", fullURL)
}

func (s *Server) HandleServe(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")

	path, ok := s.store.GetFile(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, path)
}
