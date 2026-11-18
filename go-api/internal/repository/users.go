package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailAlreadyExists = errors.New("email already exists")

type NewResident struct {
	LastName     string
	FirstName    string
	MiddleName   string
	Phone        string
	Email        string
	PasswordHash string
}

func (repo *Repository) CreateResident(ctx context.Context, user NewResident) (int64, error) {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("начать транзакцию: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID int64

	err = tx.QueryRow(ctx, `
		INSERT INTO users (
			last_name,
			first_name,
			middle_name,
			phone,
			email,
			password_hash
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6)
		RETURNING id
	`,
		user.LastName,
		user.FirstName,
		user.MiddleName,
		user.Phone,
		user.Email,
		user.PasswordHash,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
	
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "users_email_key" {
			return 0, ErrEmailAlreadyExists
		}
	
		return 0, fmt.Errorf("создать пользователя: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id
		FROM roles
		WHERE name = 'resident'
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("назначить роль жителя: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return 0, fmt.Errorf("роль resident не найдена")
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("завершить транзакцию: %w", err)
	}

	return userID, nil
}

type UserCredentials struct {
	ID           int64
	PasswordHash string
	IsActive     bool
}

func (repo *Repository) GetUserCredentialsByEmail(
	ctx context.Context,
	email string,
) (UserCredentials, error) {
	var user UserCredentials

	err := repo.db.QueryRow(ctx, `
		SELECT id, password_hash, is_active
		FROM users
		WHERE email = $1
	`, email).Scan(
		&user.ID,
		&user.PasswordHash,
		&user.IsActive,
	)
	if err != nil {
		return UserCredentials{}, fmt.Errorf("найти пользователя для входа: %w", err)
	}

	return user, nil
}

