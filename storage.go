package main

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
)

type DiskStore struct {
	BaseDir string
}

// IdByteLength must not be less than 3, or the sharding logic will panic (minimum 5 for sensible file names)
const IdByteLength = 6

func (s *DiskStore) SaveFile(r io.Reader, ext string) (string, error) {
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
	if len(id) != base64.URLEncoding.EncodedLen(IdByteLength) {
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

func generateId(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buffer), nil
}
