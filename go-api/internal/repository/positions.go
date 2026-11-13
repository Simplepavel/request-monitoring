package repository

import "context"

type Position struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (repo *Repository) GetPositions(ctx context.Context) ([]Position, error) {
	query := `SELECT id, name FROM positions ORDER BY id`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := make([]Position, 0)

	for rows.Next() {
		var position Position

		err := rows.Scan(&position.ID, &position.Name)
		if err != nil {
			return nil, err
		}

		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return positions, nil
}