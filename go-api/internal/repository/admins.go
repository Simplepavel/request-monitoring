package repository

import (
	"context"
	"errors"
	"fmt"
)

func (repo *Repository) CanManageDormitory(
	ctx context.Context,
	managerID int64,
	dormitoryID int64,
) (bool, error) {
	var canManage bool

	err := repo.db.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN roles r ON r.id = ur.role_id
				JOIN super_admin_profiles sap ON sap.user_id = ur.user_id
				WHERE ur.user_id = $1
				  AND r.name = 'super_admin'
				  AND sap.is_active
			)
			OR EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN roles r ON r.id = ur.role_id
				JOIN admin_profiles ap ON ap.user_id = ur.user_id
				WHERE ur.user_id = $1
				  AND r.name = 'admin'
				  AND ap.is_active
				  AND ap.dormitory_id = $2
			)
	`, managerID, dormitoryID).Scan(&canManage)
	if err != nil {
		return false, fmt.Errorf("проверить права администратора: %w", err)
	}

	return canManage, nil
}

type ExecutorAssignment struct {
	UserID      int64
	DormitoryID int64
	PositionID  int64
}

func (repo *Repository) AssignExecutor(
	ctx context.Context,
	assignment ExecutorAssignment,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию назначения исполнителя: %w", err)
	}
	defer tx.Rollback(ctx)

	var executorRoleID int64

	err = tx.QueryRow(ctx, `
		SELECT id
		FROM roles
		WHERE name = 'executor'
	`).Scan(&executorRoleID)
	if err != nil {
		return fmt.Errorf("найти роль executor: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, assignment.UserID, executorRoleID)
	if err != nil {
		return fmt.Errorf("назначить роль executor: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO executor_profiles (user_id, dormitory_id, position_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET dormitory_id = EXCLUDED.dormitory_id,
		    position_id = EXCLUDED.position_id
	`, assignment.UserID, assignment.DormitoryID, assignment.PositionID)
	if err != nil {
		return fmt.Errorf("создать профиль исполнителя: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить назначение исполнителя: %w", err)
	}

	return nil
}

type AdminAssignment struct {
	UserID      int64
	DormitoryID int64
}

func (repo *Repository) AssignAdmin(
	ctx context.Context,
	assignment AdminAssignment,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию назначения администратора: %w", err)
	}
	defer tx.Rollback(ctx)

	var adminRoleID int64

	err = tx.QueryRow(ctx, `
		SELECT id
		FROM roles
		WHERE name = 'admin'
	`).Scan(&adminRoleID)
	if err != nil {
		return fmt.Errorf("найти роль admin: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, assignment.UserID, adminRoleID)
	if err != nil {
		return fmt.Errorf("назначить роль admin: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO admin_profiles (user_id, dormitory_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET dormitory_id = EXCLUDED.dormitory_id,
		    is_active = TRUE,
		    deactivated_at = NULL
	`, assignment.UserID, assignment.DormitoryID)
	if err != nil {
		return fmt.Errorf("создать профиль администратора: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить назначение администратора: %w", err)
	}

	return nil
}

func (repo *Repository) IsActiveSuperAdmin(
	ctx context.Context,
	userID int64,
) (bool, error) {
	var isActive bool

	err := repo.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			JOIN super_admin_profiles sap ON sap.user_id = ur.user_id
			WHERE ur.user_id = $1
			  AND r.name = 'super_admin'
			  AND sap.is_active
		)
	`, userID).Scan(&isActive)
	if err != nil {
		return false, fmt.Errorf("проверить права супер-администратора: %w", err)
	}

	return isActive, nil
}

var ErrExecutorUnavailable = errors.New("executor is unavailable")

type ExecutorDeactivation struct {
	UserID      int64
	DormitoryID int64
}

func (repo *Repository) DeactivateExecutor(
	ctx context.Context,
	deactivation ExecutorDeactivation,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию деактивации исполнителя: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE executor_profiles
		SET is_active = FALSE,
		    deactivated_at = NOW()
		WHERE user_id = $1
		  AND dormitory_id = $2
		  AND is_active
	`, deactivation.UserID, deactivation.DormitoryID)
	if err != nil {
		return fmt.Errorf("деактивировать профиль исполнителя: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrExecutorUnavailable
	}

	_, err = tx.Exec(ctx, `
		WITH system_transitions AS (
			SELECT
				transition.from_status_id,
				transition.to_status_id
			FROM status_transitions transition
			JOIN ticket_statuses from_status
				ON from_status.id = transition.from_status_id
			JOIN ticket_statuses to_status
				ON to_status.id = transition.to_status_id
			WHERE transition.actor_role_id IS NULL
			  AND transition.comment_required = FALSE
			  AND to_status.name = 'pending_review'
			  AND from_status.name IN (
				  'in_progress',
				  'rejected_by_customer'
			  )
		),
		returned_tickets AS (
			UPDATE tickets AS ticket
			SET status_id = transition.to_status_id,
			    assignee_id = NULL,
			    finished_at = NULL
			FROM system_transitions transition
			WHERE ticket.assignee_id = $1
			  AND ticket.status_id = transition.from_status_id
			RETURNING
				ticket.id,
				transition.from_status_id,
				transition.to_status_id
		)
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by
		)
		SELECT
			returned.id,
			returned.from_status_id,
			returned.to_status_id,
			NULL
		FROM returned_tickets returned
	`, deactivation.UserID)
	if err != nil {
		return fmt.Errorf("вернуть заявки деактивированного исполнителя в очередь: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить деактивацию исполнителя: %w", err)
	}

	return nil
}

type ExecutorActivation struct {
	UserID      int64
	DormitoryID int64
}

func (repo *Repository) ActivateExecutor(
	ctx context.Context,
	activation ExecutorActivation,
) error {
	tag, err := repo.db.Exec(ctx, `
		UPDATE executor_profiles
		SET is_active = TRUE,
		    deactivated_at = NULL
		WHERE user_id = $1
		  AND dormitory_id = $2
		  AND is_active = FALSE
	`,
		activation.UserID,
		activation.DormitoryID,
	)
	if err != nil {
		return fmt.Errorf("активировать профиль исполнителя: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrExecutorUnavailable
	}

	return nil
}

type ExecutorListItem struct {
	UserID        int64  `json:"user_id"`
	LastName      string `json:"last_name"`
	FirstName     string `json:"first_name"`
	MiddleName    string `json:"middle_name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	DormitoryID   int64  `json:"dormitory_id"`
	Dormitory     int    `json:"dormitory_number"`
	PositionID    int64  `json:"position_id"`
	Position      string `json:"position_name"`
	IsActive      bool   `json:"is_active"`
	AccountActive bool   `json:"account_active"`
}

func (repo *Repository) ListExecutors(
	ctx context.Context,
	dormitoryID *int64,
) ([]ExecutorListItem, error) {
	var dormitoryFilter any
	if dormitoryID != nil {
		dormitoryFilter = *dormitoryID
	}

	rows, err := repo.db.Query(ctx, `
		SELECT
			ep.user_id,
			u.last_name,
			u.first_name,
			COALESCE(u.middle_name, ''),
			u.phone,
			u.email,
			ep.dormitory_id,
			d.number,
			ep.position_id,
			p.name,
			ep.is_active,
			u.is_active
		FROM executor_profiles ep
		JOIN users u ON u.id = ep.user_id
		JOIN dormitories d ON d.id = ep.dormitory_id
		JOIN positions p ON p.id = ep.position_id
		WHERE $1::BIGINT IS NULL OR ep.dormitory_id = $1
		ORDER BY d.number, p.name, u.last_name, u.first_name, ep.user_id
	`, dormitoryFilter)
	if err != nil {
		return nil, fmt.Errorf("получить список исполнителей: %w", err)
	}
	defer rows.Close()

	executors := make([]ExecutorListItem, 0)
	for rows.Next() {
		var executor ExecutorListItem

		err := rows.Scan(
			&executor.UserID,
			&executor.LastName,
			&executor.FirstName,
			&executor.MiddleName,
			&executor.Phone,
			&executor.Email,
			&executor.DormitoryID,
			&executor.Dormitory,
			&executor.PositionID,
			&executor.Position,
			&executor.IsActive,
			&executor.AccountActive,
		)
		if err != nil {
			return nil, fmt.Errorf("прочитать исполнителя: %w", err)
		}

		executors = append(executors, executor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обойти список исполнителей: %w", err)
	}

	return executors, nil
}

func (repo *Repository) GetActiveAdminDormitoryID(
	ctx context.Context,
	managerID int64,
) (int64, bool, error) {
	var dormitoryID int64

	err := repo.db.QueryRow(ctx, `
		SELECT COALESCE(
			(
				SELECT ap.dormitory_id
				FROM admin_profiles ap
				JOIN user_roles ur ON ur.user_id = ap.user_id
				JOIN roles r ON r.id = ur.role_id
				WHERE ap.user_id = $1
				  AND ap.is_active
				  AND r.name = 'admin'
				LIMIT 1
			),
			0
		)
	`, managerID).Scan(&dormitoryID)
	if err != nil {
		return 0, false, fmt.Errorf("получить общежитие администратора: %w", err)
	}

	if dormitoryID == 0 {
		return 0, false, nil
	}

	return dormitoryID, true, nil
}
