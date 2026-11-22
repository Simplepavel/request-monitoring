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

func (repo *Repository) ConfirmTicket(
	ctx context.Context,
	ticketID int64,
	residentID int64,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию подтверждения заявки: %w", err)
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
		WHERE from_status.name = 'awaiting_confirmation'
		  AND to_status.name = 'completed'
		  AND role.name = 'resident'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу completed: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET status_id = $1
		WHERE ticket.id = $2
		  AND ticket.author_id = $3
		  AND ticket.status_id = $4
		  AND EXISTS (
			  SELECT 1
			  FROM users user_account
			  JOIN user_roles user_role
				  ON user_role.user_id = user_account.id
			  JOIN roles role
				  ON role.id = user_role.role_id
			  WHERE user_account.id = $3
			    AND user_account.is_active
			    AND role.name = 'resident'
		  )
	`, toStatusID, ticketID, residentID, fromStatusID)
	if err != nil {
		return fmt.Errorf("подтвердить выполнение заявки: %w", err)
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
	`, ticketID, fromStatusID, toStatusID, residentID)
	if err != nil {
		return fmt.Errorf("сохранить историю подтверждения заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить подтверждение заявки: %w", err)
	}

	return nil
}

func (repo *Repository) ReturnTicketForRework(
	ctx context.Context,
	ticketID int64,
	residentID int64,
	comment string,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию возврата заявки на доработку: %w", err)
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
		WHERE from_status.name = 'awaiting_confirmation'
		  AND to_status.name = 'rejected_by_customer'
		  AND role.name = 'resident'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу rejected_by_customer: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET status_id = $1,
		    finished_at = NULL
		WHERE ticket.id = $2
		  AND ticket.author_id = $3
		  AND ticket.status_id = $4
		  AND ticket.assignee_id IS NOT NULL
		  AND EXISTS (
			  SELECT 1
			  FROM users user_account
			  JOIN user_roles user_role
				  ON user_role.user_id = user_account.id
			  JOIN roles role
				  ON role.id = user_role.role_id
			  WHERE user_account.id = $3
			    AND user_account.is_active
			    AND role.name = 'resident'
		  )
	`, toStatusID, ticketID, residentID, fromStatusID)
	if err != nil {
		return fmt.Errorf("вернуть заявку на доработку: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrTicketUnavailable
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by,
			comment
		)
		VALUES ($1, $2, $3, $4, $5)
	`, ticketID, fromStatusID, toStatusID, residentID, comment)
	if err != nil {
		return fmt.Errorf("сохранить историю возврата заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить возврат заявки на доработку: %w", err)
	}

	return nil
}

type AssignedTicket struct {
	ID                  int64
	DormitoryNumber     int
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionName        string
	Title               string
	Description         string
	StatusName          string
	CreatedAt           time.Time
}

func (repo *Repository) GetAssignedTickets(
	ctx context.Context,
	executorID int64,
) ([]AssignedTicket, error) {
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
			s.name,
			t.created_at
		FROM tickets t
		JOIN users u ON u.id = t.assignee_id
		JOIN executor_profiles ep ON ep.user_id = u.id
		JOIN ticket_statuses s ON s.id = t.status_id
		JOIN dormitories d ON d.id = t.dormitory_id
		JOIN positions p ON p.id = t.position_id
		WHERE t.assignee_id = $1
		  AND u.is_active
		  AND ep.is_active
		  AND s.name IN (
			  'in_progress',
			  'awaiting_confirmation',
			  'rejected_by_customer'
		  )
		  AND EXISTS (
			  SELECT 1
			  FROM user_roles ur
			  JOIN roles r ON r.id = ur.role_id
			  WHERE ur.user_id = u.id
			    AND r.name = 'executor'
		  )
		ORDER BY t.created_at
	`, executorID)
	if err != nil {
		return nil, fmt.Errorf("получить назначенные заявки исполнителя: %w", err)
	}
	defer rows.Close()

	tickets := make([]AssignedTicket, 0)

	for rows.Next() {
		var ticket AssignedTicket

		err := rows.Scan(
			&ticket.ID,
			&ticket.DormitoryNumber,
			&ticket.BlockNumber,
			&ticket.RoomLetter,
			&ticket.LocationDescription,
			&ticket.PositionName,
			&ticket.Title,
			&ticket.Description,
			&ticket.StatusName,
			&ticket.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("прочитать назначенную заявку: %w", err)
		}

		tickets = append(tickets, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать назначенные заявки: %w", err)
	}

	return tickets, nil
}

func (repo *Repository) ResumeTicket(
	ctx context.Context,
	ticketID int64,
	executorID int64,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию возобновления заявки: %w", err)
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
		WHERE from_status.name = 'rejected_by_customer'
		  AND to_status.name = 'in_progress'
		  AND role.name = 'executor'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу in_progress: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET status_id = $1,
		    finished_at = NULL
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
		return fmt.Errorf("возобновить работу по заявке: %w", err)
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
		return fmt.Errorf("сохранить историю возобновления заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить возобновление заявки: %w", err)
	}

	return nil
}

func (repo *Repository) RejectTicket(
	ctx context.Context,
	ticketID int64,
	executorID int64,
	comment string,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию отклонения заявки: %w", err)
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
		  AND to_status.name = 'rejected_by_executor'
		  AND role.name = 'executor'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу rejected_by_executor: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET status_id = $1,
		    finished_at = NULL
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
		return fmt.Errorf("отклонить заявку: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return ErrTicketUnavailable
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by,
			comment
		)
		VALUES ($1, $2, $3, $4, $5)
	`, ticketID, fromStatusID, toStatusID, executorID, comment)
	if err != nil {
		return fmt.Errorf("сохранить историю отклонения заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("сохранить отклонение заявки: %w", err)
	}

	return nil
}

type TicketUpdate struct {
	DormitoryID         int64
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionID          int64
	Title               string
	Description         string
}

func (repo *Repository) ResubmitTicket(
	ctx context.Context,
	ticketID int64,
	residentID int64,
	update TicketUpdate,
) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начать транзакцию повторной отправки заявки: %w", err)
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
		WHERE from_status.name = 'rejected_by_executor'
		  AND to_status.name = 'pending_review'
		  AND role.name = 'resident'
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return fmt.Errorf("найти переход к статусу pending_review: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE tickets AS ticket
		SET dormitory_id = $1,
		    block_number = $2,
		    room_letter = NULLIF($3, ''),
		    location_description = NULLIF($4, ''),
		    position_id = $5,
		    title = NULLIF($6, ''),
		    description = $7,
		    status_id = $8,
		    assignee_id = NULL,
		    finished_at = NULL
		WHERE ticket.id = $9
		  AND ticket.author_id = $10
		  AND ticket.status_id = $11
		  AND EXISTS (
			  SELECT 1
			  FROM users user_account
			  JOIN user_roles user_role
				  ON user_role.user_id = user_account.id
			  JOIN roles role
				  ON role.id = user_role.role_id
			  WHERE user_account.id = $10
			    AND user_account.is_active
			    AND role.name = 'resident'
		  )
	`,
		update.DormitoryID,
		update.BlockNumber,
		update.RoomLetter,
		update.LocationDescription,
		update.PositionID,
		update.Title,
		update.Description,
		toStatusID,
		ticketID,
		residentID,
		fromStatusID,
	)
	if err != nil {
		return fmt.Errorf("обновить и повторно отправить заявку: %w", err)
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
	`, ticketID, fromStatusID, toStatusID, residentID)
	if err != nil {
		return fmt.Errorf("сохранить историю повторной отправки заявки: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("завершить повторную отправку заявки: %w", err)
	}

	return nil
}

type ResidentTicket struct {
	ID                  int64
	DormitoryNumber     int
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionName        string
	Title               string
	Description         string
	StatusName          string
	CreatedAt           time.Time
}

func (repo *Repository) GetResidentTickets(
	ctx context.Context,
	residentID int64,
) ([]ResidentTicket, error) {
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
			s.name,
			t.created_at
		FROM tickets t
		JOIN users u ON u.id = t.author_id
		JOIN dormitories d ON d.id = t.dormitory_id
		JOIN positions p ON p.id = t.position_id
		JOIN ticket_statuses s ON s.id = t.status_id
		WHERE t.author_id = $1
		  AND u.is_active
		  AND EXISTS (
			  SELECT 1
			  FROM user_roles ur
			  JOIN roles r ON r.id = ur.role_id
			  WHERE ur.user_id = t.author_id
			    AND r.name = 'resident'
		  )
		ORDER BY t.created_at DESC, t.id DESC
	`, residentID)
	if err != nil {
		return nil, fmt.Errorf("получить заявки жителя: %w", err)
	}
	defer rows.Close()

	tickets := make([]ResidentTicket, 0)

	for rows.Next() {
		var ticket ResidentTicket

		err := rows.Scan(
			&ticket.ID,
			&ticket.DormitoryNumber,
			&ticket.BlockNumber,
			&ticket.RoomLetter,
			&ticket.LocationDescription,
			&ticket.PositionName,
			&ticket.Title,
			&ticket.Description,
			&ticket.StatusName,
			&ticket.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("прочитать заявку жителя: %w", err)
		}

		tickets = append(tickets, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать список заявок жителя: %w", err)
	}

	return tickets, nil
}

type TicketHistoryEvent struct {
	FromStatus string
	ToStatus   string
	ChangedBy  int64
	Comment    string
	ChangedAt  time.Time
}

func (repo *Repository) GetTicketHistory(
	ctx context.Context,
	ticketID int64,
	userID int64,
) ([]TicketHistoryEvent, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			COALESCE(from_status.name, ''),
			to_status.name,
			history.changed_by,
			COALESCE(history.comment, ''),
			history.changed_at
		FROM ticket_status_history history
		JOIN tickets ticket
			ON ticket.id = history.ticket_id
		LEFT JOIN ticket_statuses from_status
			ON from_status.id = history.from_status_id
		JOIN ticket_statuses to_status
			ON to_status.id = history.to_status_id
		WHERE history.ticket_id = $1
		  AND (
			  ticket.author_id = $2
			  OR ticket.assignee_id = $2
		  )
		ORDER BY history.changed_at
	`, ticketID, userID)
	if err != nil {
		return nil, fmt.Errorf("получить историю заявки: %w", err)
	}
	defer rows.Close()

	events := make([]TicketHistoryEvent, 0)

	for rows.Next() {
		var event TicketHistoryEvent

		err := rows.Scan(
			&event.FromStatus,
			&event.ToStatus,
			&event.ChangedBy,
			&event.Comment,
			&event.ChangedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("прочитать событие истории заявки: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать историю заявки: %w", err)
	}

	if len(events) == 0 {
		return nil, ErrTicketUnavailable
	}

	return events, nil
}

func (repo *Repository) AutoCloseExpiredTickets(
	ctx context.Context,
	confirmationTimeout time.Duration,
) (int64, error) {
	if confirmationTimeout <= 0 {
		return 0, fmt.Errorf("срок подтверждения должен быть больше нуля")
	}

	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("начать транзакцию автозакрытия заявок: %w", err)
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
		WHERE from_status.name = 'awaiting_confirmation'
		  AND to_status.name = 'completed'
		  AND transition.actor_role_id IS NULL
		  AND transition.comment_required = FALSE
	`).Scan(&fromStatusID, &toStatusID)
	if err != nil {
		return 0, fmt.Errorf("найти системный переход к статусу completed: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		WITH expired_tickets AS (
			UPDATE tickets AS ticket
			SET status_id = $1
			WHERE ticket.status_id = $2
			  AND ticket.finished_at <= NOW() - ($3::double precision * INTERVAL '1 second')
			RETURNING ticket.id
		)
		INSERT INTO ticket_status_history (
			ticket_id,
			from_status_id,
			to_status_id,
			changed_by
		)
		SELECT id, $2, $1, NULL
		FROM expired_tickets
	`, toStatusID, fromStatusID, confirmationTimeout.Seconds())
	if err != nil {
		return 0, fmt.Errorf("автоматически закрыть заявки: %w", err)
	}

	closedCount := tag.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("сохранить автозакрытие заявок: %w", err)
	}

	return closedCount, nil
}

type AdminTicket struct {
	ID                  int64
	DormitoryNumber     int
	BlockNumber         int
	RoomLetter          string
	LocationDescription string
	PositionName        string
	Title               string
	Description         string
	StatusName          string
	AuthorName          string
	AuthorEmail         string
	AssigneeName        string
	AssigneeEmail       string
	CreatedAt           time.Time
}

func (repo *Repository) GetAdminTickets(
	ctx context.Context,
	dormitoryID *int64,
) ([]AdminTicket, error) {
	var dormitoryFilter any
	if dormitoryID != nil {
		dormitoryFilter = *dormitoryID
	}

	rows, err := repo.db.Query(ctx, `
		SELECT
			t.id,
			d.number,
			t.block_number,
			t.room_letter,
			COALESCE(t.location_description, ''),
			p.name,
			t.title,
			t.description,
			s.name,
			CONCAT_WS(' ', author.last_name, author.first_name, author.middle_name),
			author.email,
			COALESCE(
				CONCAT_WS(' ', executor.last_name, executor.first_name, executor.middle_name),
				''
			),
			COALESCE(executor.email, ''),
			t.created_at
		FROM tickets t
		JOIN users author ON author.id = t.author_id
		LEFT JOIN users executor ON executor.id = t.assignee_id
		JOIN dormitories d ON d.id = t.dormitory_id
		JOIN positions p ON p.id = t.position_id
		JOIN ticket_statuses s ON s.id = t.status_id
		WHERE $1::BIGINT IS NULL OR t.dormitory_id = $1
		ORDER BY t.created_at DESC, t.id DESC
	`, dormitoryFilter)
	if err != nil {
		return nil, fmt.Errorf("получить административный список заявок: %w", err)
	}
	defer rows.Close()

	tickets := make([]AdminTicket, 0)
	for rows.Next() {
		var ticket AdminTicket

		err := rows.Scan(
			&ticket.ID,
			&ticket.DormitoryNumber,
			&ticket.BlockNumber,
			&ticket.RoomLetter,
			&ticket.LocationDescription,
			&ticket.PositionName,
			&ticket.Title,
			&ticket.Description,
			&ticket.StatusName,
			&ticket.AuthorName,
			&ticket.AuthorEmail,
			&ticket.AssigneeName,
			&ticket.AssigneeEmail,
			&ticket.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("прочитать заявку для администратора: %w", err)
		}

		tickets = append(tickets, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обойти административный список заявок: %w", err)
	}

	return tickets, nil
}
