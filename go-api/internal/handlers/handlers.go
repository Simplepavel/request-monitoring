package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"status": "ok",
	}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("ошибка отправки ответа: %v", err)
	}
}

func GetDormitories(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dormitories, err := repository.GetDormitories(ctx, db)
	if err != nil {
		log.Printf("Ошибка получения общежитий: %v", err)
		http.Error(w, "Не удалось получить общежития", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(dormitories)
	if err != nil {
		log.Printf("Ошибка отправки ответа: %v", err)
	}
}
