package suplier

import "testing"

func TestNewService(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Fatal("expected service instance")
	}
	if service.Repository() != nil {
		t.Fatal("expected nil repository by default")
	}
}
