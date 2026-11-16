package repository

import (
	"context"
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
