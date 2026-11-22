package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Simplepavel/request-monitoring/go-api/internal/middleware"
	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"github.com/Simplepavel/request-monitoring/go-api/internal/service"
	"log"
	"net/http"
	"time"
)

type assignExecutorRequest struct {
	UserID      int64 `json:"user_id"`
	DormitoryID int64 `json:"dormitory_id"`
	PositionID  int64 `json:"position_id"`
}

func (h *Handler) AssignExecutor(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req assignExecutorRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	if req.UserID <= 0 || req.DormitoryID <= 0 || req.PositionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Все ID должны быть положительными")
		return
	}

	managerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := h.adminService.AssignExecutor(
		ctx,
		managerID,
		repository.ExecutorAssignment{
			UserID:      req.UserID,
			DormitoryID: req.DormitoryID,
			PositionID:  req.PositionID,
		},
	)
	if errors.Is(err, service.ErrForbidden) {
		writeJSONError(w, http.StatusForbidden, "недостаточно прав")
		return
	}
	if err != nil {
		log.Printf("ошибка назначения исполнителя: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось назначить исполнителя")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type assignAdminRequest struct {
	UserID      int64 `json:"user_id"`
	DormitoryID int64 `json:"dormitory_id"`
}

func (h *Handler) AssignAdmin(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req assignAdminRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	if req.UserID <= 0 || req.DormitoryID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Все ID должны быть положительными")
		return
	}

	managerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := h.adminService.AssignAdmin(
		ctx,
		managerID,
		repository.AdminAssignment{
			UserID:      req.UserID,
			DormitoryID: req.DormitoryID,
		},
	)
	if errors.Is(err, service.ErrForbidden) {
		writeJSONError(w, http.StatusForbidden, "недостаточно прав")
		return
	}
	if err != nil {
		log.Printf("ошибка назначения администратора: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось назначить администратора")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type deactivateExecutorRequest struct {
	UserID      int64 `json:"user_id"`
	DormitoryID int64 `json:"dormitory_id"`
}

func (h *Handler) DeactivateExecutor(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req deactivateExecutorRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	if req.UserID <= 0 || req.DormitoryID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Все ID должны быть положительными")
		return
	}

	managerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := h.adminService.DeactivateExecutor(
		ctx,
		managerID,
		repository.ExecutorDeactivation{
			UserID:      req.UserID,
			DormitoryID: req.DormitoryID,
		},
	)
	if errors.Is(err, service.ErrForbidden) {
		writeJSONError(w, http.StatusForbidden, "недостаточно прав")
		return
	}
	if errors.Is(err, service.ErrCannotDeactivateExecutor) {
		writeJSONError(w, http.StatusNotFound, "исполнитель не найден или уже деактивирован")
		return
	}
	if err != nil {
		log.Printf("ошибка деактивации исполнителя: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось деактивировать исполнителя")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type activateExecutorRequest struct {
	UserID      int64 `json:"user_id"`
	DormitoryID int64 `json:"dormitory_id"`
}

func (h *Handler) ActivateExecutor(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req activateExecutorRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	if req.UserID <= 0 || req.DormitoryID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Все ID должны быть положительными")
		return
	}

	managerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := h.adminService.ActivateExecutor(
		ctx,
		managerID,
		repository.ExecutorActivation{
			UserID:      req.UserID,
			DormitoryID: req.DormitoryID,
		},
	)
	if errors.Is(err, service.ErrForbidden) {
		writeJSONError(w, http.StatusForbidden, "недостаточно прав")
		return
	}
	if errors.Is(err, service.ErrCannotActivateExecutor) {
		writeJSONError(w, http.StatusConflict, "исполнитель не найден в этом общежитии или уже активен")
		return
	}
	if err != nil {
		log.Printf("ошибка активации исполнителя: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось активировать исполнителя")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListExecutors(w http.ResponseWriter, r *http.Request) {
	managerID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	executors, err := h.adminService.ListExecutors(ctx, managerID)
	if errors.Is(err, service.ErrForbidden) {
		writeJSONError(w, http.StatusForbidden, "недостаточно прав")
		return
	}
	if err != nil {
		log.Printf("ошибка получения списка исполнителей: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось получить список исполнителей")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(executors); err != nil {
		log.Printf("ошибка отправки списка исполнителей: %v", err)
	}
}
