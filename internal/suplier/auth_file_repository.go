package suplier

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
)

type AuthFileRepository struct {
	path     string
	lockPath string
	mu       sync.Mutex
}

func NewAuthFileRepository(path string) *AuthFileRepository {
	return &AuthFileRepository{
		path:     path,
		lockPath: path + ".lock",
	}
}

func (r *AuthFileRepository) FindByPhone(_ context.Context, phone string) (SupplierAuth, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.lock()
	if err != nil {
		return SupplierAuth{}, false, err
	}
	defer unlock()

	items, err := r.readAllLocked()
	if err != nil {
		return SupplierAuth{}, false, err
	}
	index := sort.Search(len(items), func(i int) bool {
		return items[i].Phone >= phone
	})
	if index < len(items) && items[index].Phone == phone {
		return items[index], true, nil
	}
	return SupplierAuth{}, false, nil
}

func (r *AuthFileRepository) List(_ context.Context) ([]SupplierAuth, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	return r.readAllLocked()
}

func (r *AuthFileRepository) Upsert(_ context.Context, auth SupplierAuth) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.lock()
	if err != nil {
		return err
	}
	defer unlock()

	items, err := r.readAllLocked()
	if err != nil {
		return err
	}

	index := sort.Search(len(items), func(i int) bool {
		return items[i].Phone >= auth.Phone
	})
	if index < len(items) && items[index].Phone == auth.Phone {
		items[index] = auth
	} else {
		items = append(items, auth)
	}

	return r.writeAllLocked(items)
}

func (r *AuthFileRepository) readAllLocked() ([]SupplierAuth, error) {
	raw, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SupplierAuth{}, nil
		}
		return nil, err
	}
	return decodeSupplierAuths(raw), nil
}

func (r *AuthFileRepository) writeAllLocked(items []SupplierAuth) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Phone < items[j].Phone
	})
	return writeAtomically(r.path, encodeSupplierAuths(items))
}

func (r *AuthFileRepository) lock() (func() error, error) {
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
