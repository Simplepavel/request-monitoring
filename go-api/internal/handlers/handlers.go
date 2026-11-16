package handlers

import (
	"context"
	"encoding/json"
	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"github.com/Simplepavel/request-monitoring/go-api/internal/service"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	repo         *repository.Repository
	authService  *service.AuthService
	adminService *service.AdminService
}

func New(repo *repository.Repository, authService *service.AuthService, adminService *service.AdminService) *Handler {
	return &Handler{
		repo:         repo,
		authService:  authService,
		adminService: adminService,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"status": "ok",
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("ошибка отправки ответа: %v", err)
	}
}

func (h *Handler) GetDormitories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dormitories, err := h.repo.GetDormitories(ctx)
	if err != nil {
		log.Printf("ошибка получения общежитий: %v", err)
		http.Error(w, "Не удалось получить общежития", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(dormitories)
	if err != nil {
		log.Printf("ошибка отправки ответа: %v", err)
	}
}

func (h *Handler) GetPositions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	positions, err := h.repo.GetPositions(ctx)
	if err != nil {
		log.Printf("Ошибка получения специальностей: %v", err)
		http.Error(w, "Не удалось получить специальности", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(positions)
	if err != nil {
		log.Printf("Ошибка отправки ответа: %v", err)
	}
}
