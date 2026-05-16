package lyrics

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"

	"amllhub/backend/internal/model"
)

const largeFileThreshold = 10 * 1024 * 1024

func Scan(root string) ([]model.FileMeta, error) {
	var files []model.FileMeta
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		meta, err := scanFile(path, rel, info)
		if err != nil {
			return err
		}
		files = append(files, meta)
		return nil
	})
	return files, err
}

func ScanOne(root, fullPath string) (model.FileMeta, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return model.FileMeta{}, err
	}
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return model.FileMeta{}, err
	}
	return scanFile(fullPath, rel, info)
}

func scanFile(path, rel string, info os.FileInfo) (model.FileMeta, error) {
	hash, content, err := hashFile(path, info.Size())
	if err != nil {
		return model.FileMeta{}, err
	}
	return model.FileMeta{
		Path:        rel,
		ContentHash: hash,
		ModifiedAt:  info.ModTime().UTC(),
		Size:        info.Size(),
		Content:     content,
	}, nil
}

func hashFile(path string, size int64) (string, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	h := sha256.New()
	var content []byte
	if size > largeFileThreshold {
		if _, err := io.Copy(h, f); err != nil {
			return "", nil, err
		}
		return hex.EncodeToString(h.Sum(nil)), nil, nil
	}

	content, err = io.ReadAll(f)
	if err != nil {
		return "", nil, err
	}
	if _, err := h.Write(content); err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(h.Sum(nil)), content, nil
}
