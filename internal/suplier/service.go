package suplier

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

	existing, err := s.repository.List(ctx)
	if err != nil {
		return Supplier{}, err
	}
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item.Phone), normalizedPhone) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Name), normalizedName) {
			return Supplier{}, fmt.Errorf("Supplier nomi allaqachon mavjud")
		}
	}

	if err := s.repository.Add(ctx, supplier); err != nil {
		return Supplier{}, err
	}
	return supplier, nil
}

func (s *Service) FindByPhone(ctx context.Context, phone string) (Supplier, bool, error) {
	if s.repository == nil {
		return Supplier{}, false, fmt.Errorf("supplier repository is not configured")
	}

	normalizedPhone, err := NormalizePhone(phone)
	if err != nil {
		return Supplier{}, false, err
	}
	return s.repository.FindByPhone(ctx, normalizedPhone)
}

func (s *Service) List(ctx context.Context) ([]Supplier, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("supplier repository is not configured")
	}
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Phone < items[j].Phone
	})
	return items, nil
}
