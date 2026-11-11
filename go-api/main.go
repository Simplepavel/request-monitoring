package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Simplepavel/request-monitoring/go-api/internal/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

)

func main() {
	godotenv.Load("../.env")

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		log.Fatal("не задана переменная датабейс")

	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal("Ошибка настройки подключения к БД: ", err)
	}
	defer db.Close()


	err = db.Ping(ctx)
	if err != nil {
		log.Fatal("не удалось подключиться к БД: ", err)
	}

	log.Println("подключение к PostgreSQL установлено")



	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /dormitories", func(w http.ResponseWriter, r *http.Request) {
		handlers.GetDormitories(w, r, db)
	})

	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal(err)
	}
}
