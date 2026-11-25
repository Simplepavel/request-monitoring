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

type CurrentUser struct {
	ID            int64    `json:"user_id"`
	LastName      string   `json:"last_name"`
	FirstName     string   `json:"first_name"`
	MiddleName    string   `json:"middle_name"`
	Phone         string   `json:"phone"`
	Email         string   `json:"email"`
	Roles         []string `json:"roles"`
	AccountActive bool     `json:"account_active"`
}

func (repo *Repository) GetCurrentUser(
	ctx context.Context,
	userID int64,
) (CurrentUser, error) {
	var user CurrentUser

	err := repo.db.QueryRow(ctx, `
		SELECT
			u.id,
			u.last_name,
			u.first_name,
			COALESCE(u.middle_name, ''),
			u.phone,
			u.email,
			u.is_active,
			COALESCE(
				(
					SELECT array_agg(r.name::text ORDER BY r.name)
					FROM user_roles ur
					JOIN roles r ON r.id = ur.role_id
					WHERE ur.user_id = u.id
					  AND u.is_active
					  AND (
						r.name = 'resident'
						OR (r.name = 'executor' AND ep.is_active)
						OR (r.name = 'admin' AND ap.is_active)
						OR (r.name = 'super_admin' AND sap.is_active)
					  )
				),
				ARRAY[]::text[]
			)
		FROM users u
		LEFT JOIN executor_profiles ep ON ep.user_id = u.id
		LEFT JOIN admin_profiles ap ON ap.user_id = u.id
		LEFT JOIN super_admin_profiles sap ON sap.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&user.ID,
		&user.LastName,
		&user.FirstName,
		&user.MiddleName,
		&user.Phone,
		&user.Email,
		&user.AccountActive,
		&user.Roles,
	)
	if err != nil {
		return CurrentUser{}, fmt.Errorf("получить текущего пользователя: %w", err)
	}

	return user, nil
}

type SearchUserResult struct {
	ID         int64
	LastName   string
	FirstName  string
	MiddleName string
	Email      string
	Roles      []string
}

func (repo *Repository) SearchUsers(
	ctx context.Context,
	query string,
) ([]SearchUserResult, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			u.id,
			u.last_name,
			u.first_name,
			COALESCE(u.middle_name, ''),
			u.email,
			COALESCE((
				SELECT array_agg(r.name::text ORDER BY r.name)
				FROM user_roles ur
				JOIN roles r ON r.id = ur.role_id
				LEFT JOIN executor_profiles ep ON ep.user_id = ur.user_id
				LEFT JOIN admin_profiles ap ON ap.user_id = ur.user_id
				LEFT JOIN super_admin_profiles sap ON sap.user_id = ur.user_id
				WHERE ur.user_id = u.id
				  AND (
					r.name = 'resident'
					OR (r.name = 'executor' AND ep.is_active)
					OR (r.name = 'admin' AND ap.is_active)
					OR (r.name = 'super_admin' AND sap.is_active)
				  )
			), ARRAY[]::text[])
		FROM users u
		WHERE u.is_active
		  AND (
			u.email ILIKE '%' || $1 || '%'
			OR CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)
				ILIKE '%' || $1 || '%'
		  )
		ORDER BY u.last_name, u.first_name, u.id
		LIMIT 20
	`, query)
	if err != nil {
		return nil, fmt.Errorf("найти пользователей: %w", err)
	}
	defer rows.Close()

	users := make([]SearchUserResult, 0)
	for rows.Next() {
		var user SearchUserResult

		if err := rows.Scan(
			&user.ID,
			&user.LastName,
			&user.FirstName,
			&user.MiddleName,
			&user.Email,
			&user.Roles,
		); err != nil {
			return nil, fmt.Errorf("прочитать найденного пользователя: %w", err)
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обойти найденных пользователей: %w", err)
	}

	return users, nil
}
