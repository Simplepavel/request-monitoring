package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
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

func (h *Handler) AcceptTicket(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "некорректный ID заявки")
		return
	}

	executorID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = h.ticketService.AcceptTicket(ctx, executorID, ticketID)
	if errors.Is(err, service.ErrInvalidTicket) {
		writeJSONError(w, http.StatusBadRequest, "некорректный ID заявки")
		return
	}
	if errors.Is(err, service.ErrCannotAcceptTicket) {
		writeJSONError(w, http.StatusConflict, "заявку нельзя принять")
		return
	}
	if err != nil {
		log.Printf("ошибка принятия заявки: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось принять заявку")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CompleteTicket(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "некорректный ID заявки")
		return
	}

	executorID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = h.ticketService.CompleteTicket(ctx, executorID, ticketID)
	if errors.Is(err, service.ErrInvalidTicket) {
		writeJSONError(w, http.StatusBadRequest, "некорректный ID заявки")
		return
	}
	if errors.Is(err, service.ErrCannotCompleteTicket) {
		writeJSONError(w, http.StatusConflict, "заявку нельзя завершить")
		return
	}
	if err != nil {
		log.Printf("ошибка завершения заявки: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "не удалось завершить заявку")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ConfirmTicket(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Некорректный ID заявки")
		return
	}

	residentID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = h.ticketService.ConfirmTicket(ctx, residentID, ticketID)
	if errors.Is(err, service.ErrInvalidTicket) {
		writeJSONError(w, http.StatusBadRequest, "Некорректный ID заявки")
		return
	}
	if errors.Is(err, service.ErrCannotConfirmTicket) {
		writeJSONError(w, http.StatusConflict, "Заявку нельзя подтвердить")
		return
	}
	if err != nil {
		log.Printf("ошибка подтверждения заявки: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось подтвердить заявку")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type returnTicketForReworkRequest struct {
	Comment string `json:"comment"`
}

func (h *Handler) ReturnTicketForRework(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "некорректный ID заявки")
		return
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req returnTicketForReworkRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	residentID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "необходим вход в систему")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = h.ticketService.ReturnTicketForRework(
		ctx,
		residentID,
		ticketID,
		req.Comment,
	)
	if errors.Is(err, service.ErrInvalidTicket) {
		writeJSONError(w, http.StatusBadRequest, "некорректный ID заявки")
		return
	}
	if errors.Is(err, service.ErrCommentRequired) {
		writeJSONError(w, http.StatusBadRequest, "Укажите причину возврата")
		return
	}
	if errors.Is(err, service.ErrCannotReturnTicketForRework) {
		writeJSONError(w, http.StatusConflict, "заявку нельзя вернуть на доработку")
		return
	}
	if err != nil {
		log.Printf("ошибка возврата заявки на доработку: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось вернуть заявку")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
