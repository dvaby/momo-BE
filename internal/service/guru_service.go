package service

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/jwtutil"
)

type GuruService interface {
	Register(req *model.RegisterGuruRequest) (*model.Guru, error)
	Login(req *model.LoginGuruRequest) (*model.LoginGuruResponse, error)
}

type guruService struct {
	guruRepo repository.GuruRepository
}

func NewGuruService(guruRepo repository.GuruRepository) GuruService {
	return &guruService{guruRepo: guruRepo}
}

func (s *guruService) Register(req *model.RegisterGuruRequest) (*model.Guru, error) {
	existing, _ := s.guruRepo.FindByEmail(req.Email)
	if existing != nil {
		return nil, errors.New("email sudah terdaftar")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("gagal memproses password")
	}

	guru := &model.Guru{
		Nama:     req.Nama,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := s.guruRepo.Create(guru); err != nil {
		return nil, err
	}

	return guru, nil
}

func (s *guruService) Login(req *model.LoginGuruRequest) (*model.LoginGuruResponse, error) {
	guru, err := s.guruRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, errors.New("email atau password salah")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(guru.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("email atau password salah")
	}

	token, err := jwtutil.GenerateGuruToken(guru.ID)
	if err != nil {
		return nil, errors.New("gagal membuat token autentikasi")
	}

	return &model.LoginGuruResponse{
		Token: token,
		Guru:  *guru,
	}, nil
}