package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
)

var (
	ErrInvalidTicket               = errors.New("invalid ticket")
	ErrCannotAcceptTicket          = errors.New("ticket cannot be accepted")
	ErrCannotCompleteTicket        = errors.New("ticket cannot be completed")
	ErrCannotConfirmTicket         = errors.New("ticket cannot be confirmed")
	ErrCommentRequired             = errors.New("comment is required")
	ErrCannotReturnTicketForRework = errors.New("ticket cannot be returned for rework")
)

type TicketService struct {
	repo *repository.Repository
}

func NewTicketService(repo *repository.Repository) *TicketService {
	return &TicketService{
		repo: repo,
	}
}

type CreateTicketInput struct {
	DormitoryID         int64
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionID          int64
	Title               string
	Description         string
}

func (service *TicketService) CreateTicket(
	ctx context.Context,
	authorID int64,
	input CreateTicketInput,
) (int64, error) {
	if authorID <= 0 || input.DormitoryID <= 0 || input.PositionID <= 0 {
		return 0, ErrInvalidTicket
	}

	floor := input.BlockNumber / 100
	block := input.BlockNumber % 100
	if floor < 1 || block < 1 || block > 99 {
		return 0, ErrInvalidTicket
	}

	input.RoomLetter = strings.TrimSpace(input.RoomLetter)
	if input.RoomLetter != "" && input.RoomLetter != "а" && input.RoomLetter != "б" {
		return 0, ErrInvalidTicket
	}

	input.LocationDescription = strings.TrimSpace(input.LocationDescription)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if input.Description == "" {
		return 0, ErrInvalidTicket
	}

	ticketID, err := service.repo.CreateTicket(ctx, repository.NewTicket{
		AuthorID:            authorID,
		DormitoryID:         input.DormitoryID,
		BlockNumber:         input.BlockNumber,
		RoomLetter:          input.RoomLetter,
		LocationDescription: input.LocationDescription,
		PositionID:          input.PositionID,
		Title:               input.Title,
		Description:         input.Description,
	})
	if err != nil {
		return 0, fmt.Errorf("создать заявку: %w", err)
	}

	return ticketID, nil
}

func (service *TicketService) GetAvailableTickets(
	ctx context.Context,
	executorID int64,
) ([]repository.AvailableTicket, error) {
	tickets, err := service.repo.GetAvailableTickets(ctx, executorID)
	if err != nil {
		return nil, fmt.Errorf("получить очередь исполнителя: %w", err)
	}

	return tickets, nil
}

func (service *TicketService) AcceptTicket(
	ctx context.Context,
	executorID int64,
	ticketID int64,
) error {
	if executorID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	err := service.repo.AcceptTicket(ctx, ticketID, executorID)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotAcceptTicket
	}
	if err != nil {
		return fmt.Errorf("принять заявку: %w", err)
	}

	return nil
}

func (service *TicketService) CompleteTicket(
	ctx context.Context,
	executorID int64,
	ticketID int64,
) error {
	if executorID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	err := service.repo.CompleteTicket(ctx, ticketID, executorID)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotCompleteTicket
	}
	if err != nil {
		return fmt.Errorf("завершить работу по заявке: %w", err)
	}

	return nil
}

func (service *TicketService) ConfirmTicket(
	ctx context.Context,
	residentID int64,
	ticketID int64,
) error {
	if residentID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	err := service.repo.ConfirmTicket(ctx, ticketID, residentID)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotConfirmTicket
	}
	if err != nil {
		return fmt.Errorf("подтвердить выполнение заявки: %w", err)
	}

	return nil
}

func (service *TicketService) ReturnTicketForRework(
	ctx context.Context,
	residentID int64,
	ticketID int64,
	comment string,
) error {
	if residentID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	comment = strings.TrimSpace(comment)
	if comment == "" {
		return ErrCommentRequired
	}

	err := service.repo.ReturnTicketForRework(
		ctx,
		ticketID,
		residentID,
		comment,
	)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotReturnTicketForRework
	}
	if err != nil {
		return fmt.Errorf("вернуть заявку на доработку: %w", err)
	}

	return nil
}
