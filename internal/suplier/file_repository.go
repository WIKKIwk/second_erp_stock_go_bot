package suplier

import (
	"context"
	"os"
	"path/filepath"
	"sync"
)

type FileRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileRepository(path string) *FileRepository {
	return &FileRepository{path: path}
}

func (r *FileRepository) Add(_ context.Context, supplier Supplier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	return os.WriteFile(r.path, encodeSuppliers(suppliers), 0o644)
}
