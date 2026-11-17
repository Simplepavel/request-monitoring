package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Simplepavel/request-monitoring/go-api/internal/middleware"
	"github.com/Simplepavel/request-monitoring/go-api/internal/service"
)

type createTicketRequest struct {
	DormitoryID         int64  `json:"dormitory_id"`
	BlockNumber         int    `json:"block_number"`
	RoomLetter          string `json:"room_letter"`
	LocationDescription string `json:"location_description"`
	PositionID          int64  `json:"position_id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
}

type createTicketResponse struct {
	ID int64 `json:"id"`
}

func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req createTicketRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	authorID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ticketID, err := h.ticketService.CreateTicket(
		ctx,
		authorID,
		service.CreateTicketInput{
			DormitoryID:         req.DormitoryID,
			BlockNumber:         req.BlockNumber,
			RoomLetter:          req.RoomLetter,
			LocationDescription: req.LocationDescription,
			PositionID:          req.PositionID,
			Title:               req.Title,
			Description:         req.Description,
		},
	)
	if errors.Is(err, service.ErrInvalidTicket) {
		writeJSONError(w, http.StatusBadRequest, "Проверьте данные заявки")
		return
	}
	if err != nil {
		log.Printf("ошибка создания заявки: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось создать заявку")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(createTicketResponse{ID: ticketID}); err != nil {
		log.Printf("ошибка отправки ответа: %v", err)
	}
}

type availableTicketResponse struct {
	ID                  int64     `json:"id"`
	DormitoryNumber     int       `json:"dormitory_number"`
	BlockNumber         int       `json:"block_number"`
	RoomLetter          string    `json:"room_letter"`
	LocationDescription string    `json:"location_description"`
	PositionName        string    `json:"position_name"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	CreatedAt           time.Time `json:"created_at"`
}

func (h *Handler) GetAvailableTickets(w http.ResponseWriter, r *http.Request) {
	executorID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tickets, err := h.ticketService.GetAvailableTickets(ctx, executorID)
	if err != nil {
		log.Printf("ошибка получения очереди исполнителя: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось получить очередь заявок")
		return
	}

	response := make([]availableTicketResponse, 0, len(tickets))
	for _, ticket := range tickets {
		response = append(response, availableTicketResponse{
			ID:                  ticket.ID,
			DormitoryNumber:     ticket.DormitoryNumber,
			BlockNumber:         ticket.BlockNumber,
			RoomLetter:          ticket.RoomLetter,
			LocationDescription: ticket.LocationDescription,
			PositionName:        ticket.PositionName,
			Title:               ticket.Title,
			Description:         ticket.Description,
			CreatedAt:           ticket.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("ошибка отправки очереди заявок: %v", err)
	}
}
