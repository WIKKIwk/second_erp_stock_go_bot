package suplier

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFileRepositoryAddAndList(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "suppliers.fb"))

	err := repository.Add(context.Background(), Supplier{Name: "Ali", Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	err = repository.Add(context.Background(), Supplier{Name: "Vali", Phone: "+998901111111"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	items, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 suppliers, got %d", len(items))
	}
	if items[0].Phone != "+998901111111" || items[1].Phone != "+998901234567" {
		t.Fatalf("unexpected suppliers: %+v", items)
	}
}

func TestFileRepositoryFindByPhoneUsesSortedBinarySearchData(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "suppliers.fb"))

	for _, supplier := range []Supplier{
		{Name: "Vali", Phone: "+998901999999"},
		{Name: "Ali", Phone: "+998901111111"},
		{Name: "Sami", Phone: "+998901555555"},
	} {
		if err := repository.Add(context.Background(), supplier); err != nil {
			t.Fatalf("Add returned error: %v", err)
		}
	}

	supplier, ok, err := repository.FindByPhone(context.Background(), "+998901555555")
	if err != nil {
		t.Fatalf("FindByPhone returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected supplier to be found")
	}
	if supplier.Name != "Sami" {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
}

func TestFileRepositoryAddUpdatesExistingPhone(t *testing.T) {
	repository := NewFileRepository(filepath.Join(t.TempDir(), "suppliers.fb"))

	if err := repository.Add(context.Background(), Supplier{Name: "Stock", Phone: "+998901234567"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if err := repository.Add(context.Background(), Supplier{Name: "Stocker", Phone: "+998901234567"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	items, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Stocker" {
		t.Fatalf("expected existing phone to be updated, got %+v", items)
	}
}

func TestFileRepositoryConcurrentProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppliers.fb")
	processes := 6
	cmds := make([]*exec.Cmd, 0, processes)

	for i := 0; i < processes; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=TestFileRepositoryHelperProcess", "--", path, strconv.Itoa(i))
		cmd.Env = append(os.Environ(), "GO_WANT_SUPPLIER_HELPER_PROCESS=1")
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start helper process %d: %v", i, err)
		}
		cmds = append(cmds, cmd)
	}

	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("helper process %d failed: %v", i, err)
		}
	}

	repository := NewFileRepository(path)
	items, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != processes {
		t.Fatalf("expected %d suppliers, got %d: %+v", processes, len(items), items)
	}
}

func TestFileRepositoryHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SUPPLIER_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	sep := -1
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || len(args) < sep+3 {
		os.Exit(2)
	}

	path := args[sep+1]
	index, err := strconv.Atoi(args[sep+2])
	if err != nil {
		os.Exit(2)
	}

	repository := NewFileRepository(path)
	supplier := Supplier{
		Name:  fmt.Sprintf("Supplier-%d", index),
		Phone: fmt.Sprintf("+9989012345%02d", index),
	}
	if err := repository.Add(context.Background(), supplier); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
