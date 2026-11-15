package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrEmailAlreadyExists = errors.New("email already exists")
type AuthService struct {
	repo *repository.Repository
}

func NewAuthService(repo *repository.Repository) *AuthService {
	return &AuthService{
		repo: repo,
	}
}

type RegisterResidentInput struct {
	LastName   string
	FirstName  string
	MiddleName string
	Phone      string
	Email      string
	Password   string
}

func (service *AuthService) RegisterResident(
	ctx context.Context,
	input RegisterResidentInput,
) (int64, error) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(input.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return 0, fmt.Errorf("создать хэш пароля: %w", err)
	}

	user := repository.NewResident{
		LastName:     input.LastName,
		FirstName:    input.FirstName,
		MiddleName:   input.MiddleName,
		Phone:        input.Phone,
		Email:        input.Email,
		PasswordHash: string(passwordHash),
	}

	userID, err := service.repo.CreateResident(ctx, user)
	if errors.Is(err, repository.ErrEmailAlreadyExists) {
		return 0, ErrEmailAlreadyExists
	}
	if err != nil {
		return 0, fmt.Errorf("зарегистрировать жителя: %w", err)
	}

	return userID, nil
}
