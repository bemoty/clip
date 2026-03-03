package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrStorageFull = errors.New("storage limit exceeded")

type Store interface {
	SaveFile(r io.Reader, ext string, contentLength int64) (string, error)
	OpenFile(id string) (io.ReadCloser, string, bool)
	DeleteFile(id string) error
	GetMeta(id string) (time.Time, error)
	WriteMeta(id string, expiry time.Duration) error
}

type DiskStore struct {
	BaseDir      string
	maxStorageMB int64
	mu           sync.Mutex
}

// IdByteLength must not be less than 3, or the sharding logic will panic (minimum 5 for sensible file names)
const IdByteLength = 6

func NewDiskStore(ctx context.Context, config Config) *DiskStore {
	store := &DiskStore{BaseDir: config.StoragePath, maxStorageMB: config.MaxStorageMB}
	go store.sweep(ctx, config.SweepInterval)
	return store
}

func (s *DiskStore) sweep(ctx context.Context, sweepInterval time.Duration) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

func (s *DiskStore) performCleanup() {
	metaDir := filepath.Join(s.BaseDir, ".meta")
	err := filepath.WalkDir(metaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("sweep: walk error", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("sweep: failed to read meta file", "path", path, "error", err)
			return nil
		}
		expiry, err := time.Parse(time.RFC3339, string(data))
		if err != nil {
			slog.Warn("sweep: failed to parse meta file", "path", path, "error", err)
			return nil
		}
		if !time.Now().After(expiry) {
			return nil
		}
		rel, err := filepath.Rel(metaDir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 3 {
			return nil
		}
		id := parts[0] + parts[1] + parts[2]
		if err := s.DeleteFile(id); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("sweep: failed to delete expired file", "id", id, "error", err)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("sweep failed", "error", err)
	}
}

func (s *DiskStore) usedSpace() (int64, error) {
	size := int64(0)
	err := filepath.WalkDir(s.BaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(path, filepath.Join(s.BaseDir, ".meta")) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func (s *DiskStore) SaveFile(r io.Reader, ext string, contentLength int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.maxStorageMB > 0 {
		used, err := s.usedSpace()
		if err != nil {
			return "", err
		}
		limit := s.maxStorageMB << 20
		if used >= limit || (contentLength >= 0 && used+contentLength > limit) {
			return "", ErrStorageFull
		}
	}

	for {
		id, err := generateId(IdByteLength)
		if err != nil {
			return "", err
		}

		dirPath := filepath.Join(s.BaseDir, id[:2], id[2:4])
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return "", err
		}

		fileName := id[4:] + ext
		fullPath := filepath.Join(dirPath, fileName)

		file, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}

		_, err = io.Copy(file, r)
		closeErr := file.Close()

		if err != nil || closeErr != nil {
			_ = os.Remove(fullPath)
			return "", err
		}

		return id, nil
	}
}

func (s *DiskStore) GetFile(id string) (string, bool) {
	decoded, err := base64.URLEncoding.DecodeString(id)
	if len(decoded) != IdByteLength || err != nil {
		return "", false
	}

	dirPath := filepath.Join(s.BaseDir, id[:2], id[2:4])
	prefix := id[4:]
	matches, err := filepath.Glob(
		filepath.Join(dirPath, prefix+"*"),
	)
	if err != nil || len(matches) == 0 {
		return "", false
	}

	return matches[0], true
}

func (s *DiskStore) OpenFile(id string) (io.ReadCloser, string, bool) {
	path, ok := s.GetFile(id)
	if !ok {
		return nil, "", false
	}
	absBase, err := filepath.Abs(s.BaseDir)
	absPath, err2 := filepath.Abs(path)
	if err != nil || err2 != nil || !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
		return nil, "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", false
	}
	return f, filepath.Base(path), true
}

func (s *DiskStore) DeleteFile(id string) error {
	path, ok := s.GetFile(id)
	if !ok {
		_ = s.DeleteMeta(id)
		return os.ErrNotExist
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.DeleteMeta(id); err != nil {
		slog.Warn("failed to clean up meta file", "id", id, "error", err)
	}
	dir := filepath.Dir(path)
	_ = os.Remove(dir)
	_ = os.Remove(filepath.Dir(dir))
	return nil
}

func (s *DiskStore) GetMeta(id string) (time.Time, error) {
	contents, err := os.ReadFile(s.metaPath(id))
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, string(contents))
}

func (s *DiskStore) WriteMeta(id string, expiry time.Duration) error {
	path := s.metaPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Add(expiry).Format(time.RFC3339)), 0644)
}

func (s *DiskStore) DeleteMeta(id string) error {
	path := s.metaPath(id)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(path)
	_ = os.Remove(dir)
	_ = os.Remove(filepath.Dir(dir))
	return nil
}

func (s *DiskStore) metaPath(id string) string {
	return filepath.Join(s.BaseDir, ".meta", id[:2], id[2:4], id[4:])
}

func generateId(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buffer), nil
}
