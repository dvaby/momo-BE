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
}

type guruService struct {
	guruRepo    repository.GuruRepository
	emailClient *emailsender.Client // Tetap ada agar tidak error compile, tapi tidak dipakai
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

	// PERUBAHAN: Langsung set email_verified = true, skip email sending
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

	// PERUBAHAN: Hapus pengecekan email_verified agar guru bisa langsung login
	// if !guru.EmailVerified {
	// 	return nil, errors.New("email belum diverifikasi")
	// }

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
	// Fitur ini dinonaktifkan sementara
	return errors.New("verifikasi email dinonaktifkan")
}
