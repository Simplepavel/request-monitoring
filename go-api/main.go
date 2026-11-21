package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Simplepavel/request-monitoring/go-api/internal/handlers"
	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"github.com/Simplepavel/request-monitoring/go-api/internal/service"

	"github.com/Simplepavel/request-monitoring/go-api/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../.env")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("Не задана переменная DATABASE_URL")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET не задан или слишком короткий")
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
	authService := service.NewAuthService(repo, jwtSecret)
	adminService := service.NewAdminService(repo)
	ticketService := service.NewTicketService(repo)
	h := handlers.New(repo, authService, adminService, ticketService)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /dormitories", h.GetDormitories)
	mux.HandleFunc("GET /positions", h.GetPositions)
	mux.HandleFunc("POST /auth/register", h.RegisterResident)

	mux.HandleFunc("POST /auth/login", h.Login)

	mux.Handle(
		"GET /auth/me",
		middleware.RequireAuth(authService, http.HandlerFunc(h.CurrentUser)),
	)
	log.Println("HTTP-сервер запущен на порту", port)

	mux.Handle(
		"POST /admin/executors",
		middleware.RequireAuth(authService, http.HandlerFunc(h.AssignExecutor)),
	)

	mux.Handle(
		"POST /admin/admins",
		middleware.RequireAuth(authService, http.HandlerFunc(h.AssignAdmin)),
	)

	mux.Handle(
		"POST /tickets",
		middleware.RequireAuth(authService, http.HandlerFunc(h.CreateTicket)),
	)
	mux.Handle(
		"GET /tickets/available",
		middleware.RequireAuth(authService, http.HandlerFunc(h.GetAvailableTickets)),
	)
	mux.Handle(
		"POST /tickets/{id}/accept",
		middleware.RequireAuth(authService, http.HandlerFunc(h.AcceptTicket)),
	)

	mux.Handle(
		"POST /tickets/{id}/complete",
		middleware.RequireAuth(authService, http.HandlerFunc(h.CompleteTicket)),
	)

	mux.Handle(
		"POST /tickets/{id}/confirm",
		middleware.RequireAuth(authService, http.HandlerFunc(h.ConfirmTicket)),
	)

	mux.Handle(
		"POST /tickets/{id}/return",
		middleware.RequireAuth(authService, http.HandlerFunc(h.ReturnTicketForRework)),
	)
	mux.Handle(
		"GET /tickets/assigned",
		middleware.RequireAuth(authService, http.HandlerFunc(h.GetAssignedTickets)),
	)
	mux.Handle(
		"POST /tickets/{id}/resume",
		middleware.RequireAuth(authService, http.HandlerFunc(h.ResumeTicket)),
	)

	mux.Handle(
		"POST /tickets/{id}/reject",
		middleware.RequireAuth(authService, http.HandlerFunc(h.RejectTicket)),
	)

	mux.Handle(
		"POST /tickets/{id}/resubmit",
		middleware.RequireAuth(authService, http.HandlerFunc(h.ResubmitTicket)),
	)

	mux.Handle(
		"GET /tickets/my",
		middleware.RequireAuth(authService, http.HandlerFunc(h.GetResidentTickets)),
	)

	mux.Handle(
		"GET /tickets/{id}/history",
		middleware.RequireAuth(authService, http.HandlerFunc(h.GetTicketHistory)),
	)

	mux.Handle(
		"POST /admin/executors/deactivate",
		middleware.RequireAuth(authService, http.HandlerFunc(h.DeactivateExecutor)),
	)

	startTicketAutoCloser(ticketService)
	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal("Ошибка HTTP-се рвера: ", err)
	}
}

func startTicketAutoCloser(ticketService *service.TicketService) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)

			closedCount, err := ticketService.AutoCloseExpiredTickets(
				ctx,
				72*time.Hour,
			)
			cancel()

			if err != nil {
				log.Printf("ошибка автозакрытия заявок: %v", err)
			} else if closedCount > 0 {
				log.Printf("автоматически закрыто заявок: %d", closedCount)
			}

			<-ticker.C
		}
	}()
}
