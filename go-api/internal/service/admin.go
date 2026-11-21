package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
)

var (
	ErrForbidden                = errors.New("недостаточно прав")
	ErrCannotDeactivateExecutor = errors.New("исполнителя нельзя деактивировать")
)

type AdminService struct {
	repo *repository.Repository
}

func NewAdminService(repo *repository.Repository) *AdminService {
	return &AdminService{
		repo: repo,
	}
}

func (service *AdminService) AssignExecutor( //назначить исполнителя
	ctx context.Context,
	managerID int64,
	assignment repository.ExecutorAssignment,
) error {
	canManage, err := service.repo.CanManageDormitory(
		ctx,
		managerID,
		assignment.DormitoryID,
	)
	if err != nil {
		return fmt.Errorf("проверить права администратора: %w", err)
	}

	if !canManage {
		return ErrForbidden
	}

	err = service.repo.AssignExecutor(ctx, assignment)
	if err != nil {
		return fmt.Errorf("назначить исполнителя: %w", err)
	}

	return nil
}

func (service *AdminService) AssignAdmin( // назначить админа
	ctx context.Context,
	managerID int64,
	assignment repository.AdminAssignment,
) error {
	isSuperAdmin, err := service.repo.IsActiveSuperAdmin(ctx, managerID)
	if err != nil {
		return fmt.Errorf("проверить права супер-администратора: %w", err)
	}

	if !isSuperAdmin {
		return ErrForbidden
	}

	if err := service.repo.AssignAdmin(ctx, assignment); err != nil {
		return fmt.Errorf("назначить администратора: %w", err)
	}

	return nil
}

func (service *AdminService) DeactivateExecutor(
	ctx context.Context,
	managerID int64,
	deactivation repository.ExecutorDeactivation,
) error {
	canManage, err := service.repo.CanManageDormitory(
		ctx,
		managerID,
		deactivation.DormitoryID,
	)
	if err != nil {
		return fmt.Errorf("проверить права администратора: %w", err)
	}

	if !canManage {
		return ErrForbidden
	}

	err = service.repo.DeactivateExecutor(ctx, deactivation)
	if errors.Is(err, repository.ErrExecutorUnavailable) {
		return ErrCannotDeactivateExecutor
	}
	if err != nil {
		return fmt.Errorf("деактивировать исполнителя: %w", err)
	}

	return nil
}
