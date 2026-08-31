package service

import (
	"errors"

	"momo-be/internal/model"
	"momo-be/internal/repository"
)

type ModulService struct {
	repo repository.ModulRepository
}

func NewModulService(repo repository.ModulRepository) *ModulService {
	return &ModulService{repo: repo}
}

func (s *ModulService) Create(guruID uint, nama string, deskripsi string) (*model.Modul, error) {
	if nama == "" {
		return nil, errors.New("nama modul wajib diisi")
	}

	modul := &model.Modul{
		GuruID:    guruID,
		Nama:      nama,
		Deskripsi: deskripsi,
	}

	err := s.repo.Create(modul)
	if err != nil {
		return nil, err
	}

	return modul, nil
}

func (s *ModulService) GetAll() ([]model.Modul, error) {
	return s.repo.FindAll()
}

func (s *ModulService) GetByID(id uint) (*model.Modul, error) {
	return s.repo.FindByID(id)
}

func (s *ModulService) GetByIDAndGuruID(id, guruID uint) (*model.Modul, error) {
	return s.repo.FindByIDAndGuruID(id, guruID)
}

func (s *ModulService) GetByGuruID(guruID uint) ([]model.Modul, error) {
	return s.repo.FindByGuruID(guruID)
}

func (s *ModulService) Update(id uint, nama string, deskripsi string) (*model.Modul, error) {
	modul, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if nama != "" {
		modul.Nama = nama
	}
	modul.Deskripsi = deskripsi

	err = s.repo.Update(modul)
	if err != nil {
		return nil, err
	}

	return modul, nil
}

func (s *ModulService) Delete(id uint) error {
	return s.repo.Delete(id)
}