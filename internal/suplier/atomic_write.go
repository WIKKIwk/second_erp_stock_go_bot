package suplier

import (
	"os"
	"path/filepath"
)

func writeAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, ".suppliers-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	dirFile, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer dirFile.Close()
	_ = dirFile.Sync()
	return nil
}
