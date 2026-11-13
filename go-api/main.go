package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Simplepavel/request-monitoring/go-api/internal/handlers"
	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../.env")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("Не задана переменная DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		cancel()
		log.Fatal("Ошибка настройки подключения к БД: ", err)
	}

	err = db.Ping(ctx)
	cancel()

	if err != nil {
		db.Close()
		log.Fatal("Не удалось подключиться к БД: ", err)
	}
	defer db.Close()

	log.Println("Подключение к PostgreSQL установлено")

	repo := repository.New(db)
	h := handlers.New(repo)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /dormitories", h.GetDormitories)
	mux.HandleFunc("GET /positions", h.GetPositions)

	log.Println("HTTP-сервер запущен на порту", port)

	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal("Ошибка HTTP-сервера: ", err)
	}
}