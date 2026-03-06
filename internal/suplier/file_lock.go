package suplier

import (
	"os"
	"path/filepath"
	"syscall"
)

func (r *FileRepository) lock() (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(r.lockPath), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(r.lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}

	return func() error {
		unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, nil
}
