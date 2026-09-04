package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

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
	guruRepo     repository.GuruRepository
	emailClient  *emailsender.Client
	appBaseURL   string
}

func NewGuruService(guruRepo repository.GuruRepository, emailClient *emailsender.Client, appBaseURL string) GuruService {
	return &guruService{
		guruRepo:    guruRepo,
		emailClient: emailClient,
		appBaseURL:  appBaseURL,
	}
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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

	token, err := generateToken()
	if err != nil {
		return nil, errors.New("gagal membuat token verifikasi")
	}

	guru := &model.Guru{
		Nama:              req.Nama,
		Email:             req.Email,
		Password:          string(hashedPassword),
		EmailVerified:     false,
		VerificationToken: token,
	}

	if err := s.guruRepo.Create(guru); err != nil {
		return nil, err
	}

	verifyLink := fmt.Sprintf("%s/api/v1/guru/verify-email?token=%s", s.appBaseURL, token)
	if err := s.emailClient.SendVerificationEmail(guru.Email, guru.Nama, verifyLink); err != nil {
		return nil, fmt.Errorf("akun berhasil dibuat, tapi gagal mengirim email verifikasi: %w", err)
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

	if !guru.EmailVerified {
		return nil, errors.New("email belum diverifikasi, silakan cek inbox kamu")
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
	guru, err := s.guruRepo.FindByVerificationToken(token)
	if err != nil {
		return errors.New("token verifikasi tidak valid")
	}

	guru.EmailVerified = true
	guru.VerificationToken = ""

	return s.guruRepo.Update(guru)
}