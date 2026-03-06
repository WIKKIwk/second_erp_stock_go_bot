package suplier

import (
	"context"
	"fmt"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Add(ctx context.Context, name, phone string) (Supplier, error) {
	if s.repository == nil {
		return Supplier{}, fmt.Errorf("supplier repository is not configured")
	}

	normalizedName, err := NormalizeName(name)
	if err != nil {
		return Supplier{}, err
	}
	normalizedPhone, err := NormalizePhone(phone)
	if err != nil {
		return Supplier{}, err
	}

	supplier := Supplier{
		Name:  normalizedName,
		Phone: normalizedPhone,
	}
	if err := s.repository.Add(ctx, supplier); err != nil {
		return Supplier{}, err
	}
	return supplier, nil
}

func (s *Service) List(ctx context.Context) ([]Supplier, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("supplier repository is not configured")
	}
	return s.repository.List(ctx)
}
