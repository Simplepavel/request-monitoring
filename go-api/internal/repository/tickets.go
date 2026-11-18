package repository

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrTicketUnavailable = errors.New("ticket is unavailable")

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

type AvailableTicket struct {
	ID                  int64
	DormitoryNumber     int
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionName        string
	Title               string
	Description         string
	CreatedAt           time.Time
}

func (repo *Repository) GetAvailableTickets(
	ctx context.Context,
	executorID int64,
) ([]AvailableTicket, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			t.id,
			d.number,
			t.block_number,
			COALESCE(t.room_letter::text, ''),
			COALESCE(t.location_description, ''),
			p.name,
			COALESCE(t.title, ''),
			t.description,
			t.created_at
		FROM executor_profiles ep
		JOIN users u ON u.id = ep.user_id
		JOIN user_roles ur ON ur.user_id = ep.user_id
		JOIN roles r ON r.id = ur.role_id
		JOIN tickets t
			ON t.dormitory_id = ep.dormitory_id
			AND t.position_id = ep.position_id
		JOIN ticket_statuses s
			ON s.id = t.status_id
			AND s.name = 'pending_review'
		JOIN dormitories d ON d.id = t.dormitory_id
		JOIN positions p ON p.id = t.position_id
		WHERE ep.user_id = $1
		  AND ep.is_active
		  AND u.is_active
		  AND r.name = 'executor'
		  AND t.assignee_id IS NULL
		ORDER BY t.created_at
	`, executorID)
	if err != nil {
		return nil, fmt.Errorf("получить очередь исполнителя: %w", err)
	}
	defer rows.Close()

	tickets := make([]AvailableTicket, 0)

	for rows.Next() {
		var ticket AvailableTicket

		err := rows.Scan(
			&ticket.ID,
			&ticket.DormitoryNumber,
			&ticket.BlockNumber,
			&ticket.RoomLetter,
			&ticket.LocationDescription,
			&ticket.PositionName,
			&ticket.Title,
			&ticket.Description,
			&ticket.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("прочитать заявку из очереди: %w", err)
		}

		tickets = append(tickets, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать очередь исполнителя: %w", err)
	}

	return tickets, nil
}

func (repo *Repository) AcceptTicket(
	ctx context.Context,
	ticketID int64,
	executorID int64,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию принятия заявки: %w", err)
	}
	defer tx.Rollback(ctx)

	var fromStatusID int64
	var toStatusID int64

	err = tx.QueryRow(ctx, `
		SELECT transition.from_status_id, transition.to_status_id
		FROM status_transitions transition
		JOIN ticket_statuses from_status
			ON from_status.id = transition.from_status_id
		JOIN ticket_statuses to_status
			ON to_status.id = transition.to_status_id
		JOIN roles role
			ON role.id = transition.actor_role_id
		WHERE from_status.name = 'pending_review'
		  AND to_status.name = 'in_progress'
		  AND role.name = 'executor'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу in_progress: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET assignee_id = $1,
		    status_id = $2
		WHERE ticket.id = $3
		  AND ticket.status_id = $4
		  AND ticket.assignee_id IS NULL
		  AND EXISTS (
			  SELECT 1
			  FROM executor_profiles profile
			  JOIN users user_account
				  ON user_account.id = profile.user_id
			  JOIN user_roles user_role
				  ON user_role.user_id = profile.user_id
			  JOIN roles role
				  ON role.id = user_role.role_id
			  WHERE profile.user_id = $1
			    AND profile.dormitory_id = ticket.dormitory_id
			    AND profile.position_id = ticket.position_id
			    AND profile.is_active
			    AND user_account.is_active
			    AND role.name = 'executor'
		  )
	`, executorID, toStatusID, ticketID, fromStatusID)
	if err != nil {
		return fmt.Errorf("назначить исполнителя и изменить статус заявки: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrTicketUnavailable
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by
		)
		VALUES ($1, $2, $3, $4)
	`, ticketID, fromStatusID, toStatusID, executorID)
	if err != nil {
		return fmt.Errorf("сохранить историю принятия заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить принятие заявки: %w", err)
	}

	return nil
}

func (repo *Repository) CompleteTicket(
	ctx context.Context,
	ticketID int64,
	executorID int64,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию завершения заявки: %w", err)
	}
	defer tx.Rollback(ctx)

	var fromStatusID int64
	var toStatusID int64

	err = tx.QueryRow(ctx, `
		SELECT transition.from_status_id, transition.to_status_id
		FROM status_transitions transition
		JOIN ticket_statuses from_status
			ON from_status.id = transition.from_status_id
		JOIN ticket_statuses to_status
			ON to_status.id = transition.to_status_id
		JOIN roles role
			ON role.id = transition.actor_role_id
		WHERE from_status.name = 'in_progress'
		  AND to_status.name = 'awaiting_confirmation'
		  AND role.name = 'executor'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу awaiting_confirmation: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET status_id = $1,
		    finished_at = NOW()
		WHERE ticket.id = $2
		  AND ticket.status_id = $3
		  AND ticket.assignee_id = $4
		  AND EXISTS (
			  SELECT 1
			  FROM executor_profiles profile
			  JOIN users user_account
				  ON user_account.id = profile.user_id
			  JOIN user_roles user_role
				  ON user_role.user_id = profile.user_id
			  JOIN roles role
				  ON role.id = user_role.role_id
			  WHERE profile.user_id = $4
			    AND profile.dormitory_id = ticket.dormitory_id
			    AND profile.position_id = ticket.position_id
			    AND profile.is_active
			    AND user_account.is_active
			    AND role.name = 'executor'
		  )
	`, toStatusID, ticketID, fromStatusID, executorID)
	if err != nil {
		return fmt.Errorf("обновить статус заявки: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrTicketUnavailable
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by
		)
		VALUES ($1, $2, $3, $4)
	`, ticketID, fromStatusID, toStatusID, executorID)
	if err != nil {
		return fmt.Errorf("сохранить историю завершения заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить завершение заявки: %w", err)
	}

	return nil
}
