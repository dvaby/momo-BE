package service

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"momo-be/internal/model"
	"momo-be/internal/repository"
	"momo-be/pkg/emailsender"
	"momo-be/pkg/jwtutil"
)

type GuruService interface {
	Register(req *model.RegisterGuruRequest) (*model.Guru, error)
	Login(req *model.LoginGuruRequest) (*model.LoginGuruResponse, error)
	VerifyEmail(token string) error
	GetProfile(guruID uint) (*model.Guru, error)
	UpdateProfile(guruID uint, req *model.UpdateProfileRequest) (*model.Guru, error)
}

type guruService struct {
	guruRepo    repository.GuruRepository
	emailClient *emailsender.Client
	appBaseURL  string
}

func NewGuruService(guruRepo repository.GuruRepository, emailClient *emailsender.Client, appBaseURL string) GuruService {
	return &guruService{
		guruRepo:    guruRepo,
		emailClient: emailClient,
		appBaseURL:  appBaseURL,
	}
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
		Nama:              req.Nama,
		Email:             req.Email,
		Password:          string(hashedPassword),
		EmailVerified:     true,
		VerificationToken: "",
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

func (s *guruService) VerifyEmail(token string) error {
	return errors.New("verifikasi email dinonaktifkan")
}

// GetProfile - ambil data guru, password di-zero sebelum return
func (s *guruService) GetProfile(guruID uint) (*model.Guru, error) {
	guru, err := s.guruRepo.FindByID(guruID)
	if err != nil {
		return nil, errors.New("guru tidak ditemukan")
	}
	guru.Password = ""
	return guru, nil
}

// UpdateProfile - update nama dan/atau password
func (s *guruService) UpdateProfile(guruID uint, req *model.UpdateProfileRequest) (*model.Guru, error) {
	guru, err := s.guruRepo.FindByID(guruID)
	if err != nil {
		return nil, errors.New("guru tidak ditemukan")
	}

	if req.Nama == "" && req.Password == "" {
		return nil, errors.New("minimal salah satu field (nama atau password) harus diisi")
	}

	if req.Nama != "" {
		guru.Nama = req.Nama
	}

	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, errors.New("gagal memproses password")
		}
		guru.Password = string(hashedPassword)
	}

	if err := s.guruRepo.Update(guru); err != nil {
		return nil, errors.New("gagal memperbarui profil")
	}

	guru.Password = ""
	return guru, nil
}