package util

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// Wraps os.WriteFile to automatically create parent directories such that the
// caller does not need to ensure the existence of the file's directory
func WriteFileWithDir(filePath string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(path.Dir(filePath), perm); err != nil {
		return err
	}
	return os.WriteFile(filePath, data, perm)
}

// IsFilePathExists checks whether specific file path exists
func IsFilePathExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// WriteFileWithDirFromReader writes to a file from a byte reader interface
func WriteFileWithDirFromReader(path string, reader io.Reader, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), perm); err != nil {
		return err
	}
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	if _, err := io.Copy(fh, reader); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}
