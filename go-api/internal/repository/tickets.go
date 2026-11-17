package repository

import (
	"context"
	"fmt"
)

type NewTicket struct {
	AuthorID            int64
	DormitoryID         int64
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionID          int64
	Title               string
	Description         string
}

func (repo *Repository) CreateTicket(
	ctx context.Context,
	ticket NewTicket,
) (int64, error) {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("начать транзакцию создания заявки: %w", err)
	}
	defer tx.Rollback(ctx)

	var statusID int64

	err = tx.QueryRow(ctx, `
		SELECT id
		FROM ticket_statuses
		WHERE name = 'pending_review'
	`).Scan(&statusID)
	if err != nil {
		return 0, fmt.Errorf("найти начальный статус заявки: %w", err)
	}

	var ticketID int64

	err = tx.QueryRow(ctx, `
		INSERT INTO tickets (
			author_id,
			dormitory_id,
			block_number,
			room_letter,
			location_description,
			position_id,
			status_id,
			title,
			description
		)
		VALUES (
			$1,
			$2,
			$3,
			NULLIF($4, ''),
			NULLIF($5, ''),
			$6,
			$7,
			NULLIF($8, ''),
			$9
		)
		RETURNING id
	`,
		ticket.AuthorID,
		ticket.DormitoryID,
		ticket.BlockNumber,
		ticket.RoomLetter,
		ticket.LocationDescription,
		ticket.PositionID,
		statusID,
		ticket.Title,
		ticket.Description,
	).Scan(&ticketID)
	if err != nil {
		return 0, fmt.Errorf("сохранить заявку: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by
		)
		VALUES ($1, NULL, $2, $3)
	`, ticketID, statusID, ticket.AuthorID)
	if err != nil {
		return 0, fmt.Errorf("сохранить начальную историю статуса: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("завершить создание заявки: %w", err)
	}

	return ticketID, nil
}
