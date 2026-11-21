package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Simplepavel/request-monitoring/go-api/internal/repository"
	"strings"
	"time"
)

var (
	ErrInvalidTicket               = errors.New("invalid ticket")
	ErrCannotAcceptTicket          = errors.New("ticket cannot be accepted")
	ErrCannotCompleteTicket        = errors.New("ticket cannot be completed")
	ErrCannotConfirmTicket         = errors.New("ticket cannot be confirmed")
	ErrCommentRequired             = errors.New("comment is required")
	ErrCannotReturnTicketForRework = errors.New("ticket cannot be returned for rework")
	ErrCannotResumeTicket          = errors.New("ticket cannot be resumed")
	ErrCannotRejectTicket          = errors.New("ticket cannot be rejected")
	ErrCannotResubmitTicket        = errors.New("ticket cannot be resubmitted")
	ErrCannotViewTicketHistory     = errors.New("ticket history is unavailable")
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
	if authorID <= 0 {
		return 0, ErrInvalidTicket
	}

	input, err := validateAndNormalizeTicketInput(input)
	if err != nil {
		return 0, err
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

func (service *TicketService) GetAssignedTickets(
	ctx context.Context,
	executorID int64,
) ([]repository.AssignedTicket, error) {
	tickets, err := service.repo.GetAssignedTickets(ctx, executorID)
	if err != nil {
		return nil, fmt.Errorf("получить назначенные заявки исполнителя: %w", err)
	}

	return tickets, nil
}

func (service *TicketService) ResumeTicket(
	ctx context.Context,
	executorID int64,
	ticketID int64,
) error {
	if executorID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	err := service.repo.ResumeTicket(ctx, ticketID, executorID)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotResumeTicket
	}
	if err != nil {
		return fmt.Errorf("возобновить работу по заявке: %w", err)
	}

	return nil
}

func (service *TicketService) RejectTicket(
	ctx context.Context,
	executorID int64,
	ticketID int64,
	comment string,
) error {
	if executorID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	comment = strings.TrimSpace(comment)
	if comment == "" {
		return ErrCommentRequired
	}

	err := service.repo.RejectTicket(ctx, ticketID, executorID, comment)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotRejectTicket
	}
	if err != nil {
		return fmt.Errorf("отклонить заявку: %w", err)
	}

	return nil
}

func validateAndNormalizeTicketInput(
	input CreateTicketInput,
) (CreateTicketInput, error) {
	if input.DormitoryID <= 0 || input.PositionID <= 0 {
		return input, ErrInvalidTicket
	}

	floor := input.BlockNumber / 100
	block := input.BlockNumber % 100
	if floor < 1 || block < 1 || block > 99 {
		return input, ErrInvalidTicket
	}

	input.RoomLetter = strings.TrimSpace(input.RoomLetter)
	if input.RoomLetter != "" && input.RoomLetter != "а" && input.RoomLetter != "б" {
		return input, ErrInvalidTicket
	}

	input.LocationDescription = strings.TrimSpace(input.LocationDescription)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if input.Description == "" {
		return input, ErrInvalidTicket
	}

	return input, nil
}

func (service *TicketService) ResubmitTicket(
	ctx context.Context,
	residentID int64,
	ticketID int64,
	input CreateTicketInput,
) error {
	if residentID <= 0 || ticketID <= 0 {
		return ErrInvalidTicket
	}

	input, err := validateAndNormalizeTicketInput(input)
	if err != nil {
		return err
	}

	err = service.repo.ResubmitTicket(
		ctx,
		ticketID,
		residentID,
		repository.TicketUpdate{
			DormitoryID:         input.DormitoryID,
			BlockNumber:         input.BlockNumber,
			RoomLetter:          input.RoomLetter,
			LocationDescription: input.LocationDescription,
			PositionID:          input.PositionID,
			Title:               input.Title,
			Description:         input.Description,
		},
	)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return ErrCannotResubmitTicket
	}
	if err != nil {
		return fmt.Errorf("повторно отправить заявку: %w", err)
	}

	return nil
}

func (service *TicketService) GetResidentTickets(
	ctx context.Context,
	residentID int64,
) ([]repository.ResidentTicket, error) {
	if residentID <= 0 {
		return nil, ErrInvalidTicket
	}

	tickets, err := service.repo.GetResidentTickets(ctx, residentID)
	if err != nil {
		return nil, fmt.Errorf("получить заявки жителя: %w", err)
	}

	return tickets, nil
}

func (service *TicketService) GetTicketHistory(
	ctx context.Context,
	userID int64,
	ticketID int64,
) ([]repository.TicketHistoryEvent, error) {
	if userID <= 0 || ticketID <= 0 {
		return nil, ErrInvalidTicket
	}

	events, err := service.repo.GetTicketHistory(ctx, ticketID, userID)
	if errors.Is(err, repository.ErrTicketUnavailable) {
		return nil, ErrCannotViewTicketHistory
	}
	if err != nil {
		return nil, fmt.Errorf("получить историю заявки: %w", err)
	}

	return events, nil
}

func (service *TicketService) AutoCloseExpiredTickets(
	ctx context.Context,
	confirmationTimeout time.Duration,
) (int64, error) {
	closedCount, err := service.repo.AutoCloseExpiredTickets(
		ctx,
		confirmationTimeout,
	)
	if err != nil {
		return 0, fmt.Errorf("автоматически закрыть просроченные заявки: %w", err)
	}

	return closedCount, nil
}
