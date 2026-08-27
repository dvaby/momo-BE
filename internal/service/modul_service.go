package service

import (
	"errors"
	"momo-be/internal/model"
	"momo-be/internal/repository"
)

type ModulService interface {
	CreateModul(judul, deskripsi string, guruID uint) (*model.Modul, error)
	GetAllModuls(guruID uint) ([]model.Modul, error)
	GetModulByID(id uint, guruID uint) (*model.Modul, error)
}

type modulService struct {
	modulRepo repository.ModulRepository
}

func NewModulService(modulRepo repository.ModulRepository) ModulService {
	return &modulService{modulRepo}
}

func (s *modulService) CreateModul(judul, deskripsi string, guruID uint) (*model.Modul, error) {
	if judul == "" {
		return nil, errors.New("judul modul tidak boleh kosong")
	}

	modul := &model.Modul{
		GuruID:    guruID,
		Judul:     judul,
		Deskripsi: deskripsi,
	}

	err := s.modulRepo.Create(modul)
	if err != nil {
		return nil, err
	}

	return modul, nil
}

func (s *modulService) GetAllModuls(guruID uint) ([]model.Modul, error) {
	return s.modulRepo.FindAllByGuruID(guruID)
}

func (s *modulService) GetModulByID(id uint, guruID uint) (*model.Modul, error) {
	modul, err := s.modulRepo.FindByIDAndGuruID(id, guruID)
	if err != nil {
		return nil, errors.New("modul tidak ditemukan atau Anda tidak memiliki akses")
	}
	return modul, nil
}