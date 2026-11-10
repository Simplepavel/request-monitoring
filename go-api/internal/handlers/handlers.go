package handlers

import (
	"encoding/json"
	"log"
	"net/http"
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
