package service

import (
	"momo-be/internal/model"
	"momo-be/internal/repository"
)

type ModulService struct {
	repo *repository.ModulRepository
}

func NewModulService(repo *repository.ModulRepository) *ModulService {
	return &ModulService{repo: repo}
}

func (s *ModulService) CreateModul(judul string, deskripsi string) (*model.Modul, error) {
	modul := &model.Modul{
		Judul:     judul,
		Deskripsi: deskripsi,
	}

	err := s.repo.Create(modul)
	if err != nil {
		return nil, err
	}

	return modul, nil
}

func (s *ModulService) GetAllModul() ([]model.Modul, error) {
	return s.repo.FindAll()
}

func (s *ModulService) GetModulByID(id uint) (*model.Modul, error) {
	return s.repo.FindByID(id)
}