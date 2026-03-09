package suplier

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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
	replaced := false
	for i := range suppliers {
		if suppliers[i].Phone == supplier.Phone {
			suppliers[i] = supplier
			replaced = true
			break
		}
	}
	if !replaced {
		suppliers = append(suppliers, supplier)
	}
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

func (r *FileRepository) FindByPhone(_ context.Context, phone string) (Supplier, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.lock()
	if err != nil {
		return Supplier{}, false, err
	}
	defer unlock()

	suppliers, err := r.readAllLocked()
	if err != nil {
		return Supplier{}, false, err
	}
	index := sort.Search(len(suppliers), func(i int) bool {
		return suppliers[i].Phone >= phone
	})
	if index < len(suppliers) && suppliers[index].Phone == phone {
		return suppliers[index], true, nil
	}
	return Supplier{}, false, nil
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
	sort.Slice(suppliers, func(i, j int) bool {
		return suppliers[i].Phone < suppliers[j].Phone
	})
	return writeAtomically(r.path, encodeSuppliers(suppliers))
}
