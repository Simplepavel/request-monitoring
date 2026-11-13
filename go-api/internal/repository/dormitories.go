package repository

import "context"

type Dormitory struct {
	ID     int64 `json:"id"`
	Number int   `json:"number"`
}

func (repo *Repository) GetDormitories(ctx context.Context) ([]Dormitory, error) {
	query := `SELECT id, number FROM dormitories ORDER BY number`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dormitories := make([]Dormitory, 0)

	for rows.Next() {
		var dormitory Dormitory

		err := rows.Scan(&dormitory.ID, &dormitory.Number)
		if err != nil {
			return nil, err
		}

		dormitories = append(dormitories, dormitory)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dormitories, nil
}