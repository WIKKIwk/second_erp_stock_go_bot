package suplier

import (
	"context"
	"os"
	"path/filepath"
	"sync"
)

type FileRepository struct {
	path     string
	lockPath string
	mu       sync.Mutex
}

func NewFileRepository(path string) *FileRepository {
	return &FileRepository{
		path:     path,
		lockPath: path + ".lock",
	}
}

func (r *FileRepository) Add(_ context.Context, supplier Supplier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.lock()
	if err != nil {
		return err
	}
	defer unlock()

	suppliers, err := r.readAllLocked()
	if err != nil {
		return err
	}
	suppliers = append(suppliers, supplier)
	return r.writeAllLocked(suppliers)
}

func (r *FileRepository) List(_ context.Context) ([]Supplier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	return r.readAllLocked()
}

func (r *FileRepository) readAllLocked() ([]Supplier, error) {
	raw, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Supplier{}, nil
		}
		return nil, err
	}
	return decodeSuppliers(raw), nil
}

func (r *FileRepository) writeAllLocked(suppliers []Supplier) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	return writeAtomically(r.path, encodeSuppliers(suppliers))
}
