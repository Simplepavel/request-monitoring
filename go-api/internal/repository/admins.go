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
