package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"golang.org/x/crypto/bcrypt"

	"github.com/golang-jwt/jwt/v5"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid token")
)

type AuthService struct {
	repo      *repository.Repository
	jwtSecret []byte
}
type accessTokenClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func NewAuthService(repo *repository.Repository, jwtSecret string) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
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

type LoginInput struct {
	Email    string
	Password string
}

func (service *AuthService) Login(
	ctx context.Context,
	input LoginInput,
) (string, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	user, err := service.repo.GetUserCredentialsByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("получить данные для входа: %w", err)
	}

	if !user.IsActive {
		return "", ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(input.Password),
	)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	return service.createAccessToken(user.ID)
}

func (service *AuthService) createAccessToken(userID int64) (string, error) {
	now := time.Now()

	claims := accessTokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(service.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("создать токен доступа: %w", err)
	}

	return tokenString, nil
}

func (service *AuthService) UserIDFromAccessToken(tokenString string) (int64, error) {
	claims := &accessTokenClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return service.jwtSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)

	if err != nil || token == nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	if claims.UserID <= 0 {
		return 0, ErrInvalidToken
	}

	return claims.UserID, nil
}

func (service *AuthService) GetCurrentUser(
	ctx context.Context,
	userID int64,
) (repository.CurrentUser, error) {
	if userID <= 0 {
		return repository.CurrentUser{}, ErrInvalidToken
	}

	user, err := service.repo.GetCurrentUser(ctx, userID)
	if err != nil {
		return repository.CurrentUser{}, fmt.Errorf(
			"получить профиль текущего пользователя: %w",
			err,
		)
	}

	return user, nil
}
