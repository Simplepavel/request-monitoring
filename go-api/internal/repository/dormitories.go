package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Dormitory struct {
	Id int64 `json:"id"`
	Number int `json:"number"`
}

func GetDormitories(ctx context.Context, db *pgxpool.Pool) ([]Dormitory, error){
	query := `SELECT id, number FROM dormitories ORDER BY number`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dormitories := make([]Dormitory, 0)
	for rows.Next() {
		var dormitory Dormitory
		err := rows.Scan(&dormitory.Id, &dormitory.Number)
		if err != nil {
			return nil, err
		}
		dormitories = append(dormitories, dormitory)
	}

	err = rows.Err()
	if err != nil{
		return nil, err
	}

	return dormitories, nil
}


