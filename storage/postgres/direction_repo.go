package postgres

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type directionRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewDirectionRepo(db *pgxpool.Pool, log logger.ILogger) storage.IDirectionStorage {
	return &directionRepo{db: db, log: log}
}

func (r *directionRepo) GetAll(ctx context.Context) ([]*models.Direction, error) {
	query := `SELECT id, from_location, to_location, created_at FROM directions ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var directions []*models.Direction
	for rows.Next() {
		var d models.Direction
		if err := rows.Scan(&d.ID, &d.FromLocation, &d.ToLocation, &d.CreatedAt); err != nil {
			return nil, err
		}
		directions = append(directions, &d)
	}
	return directions, nil
}

func (r *directionRepo) Create(ctx context.Context, from, to string) error {
	query := `INSERT INTO directions (from_location, to_location) VALUES ($1, $2)`
	_, err := r.db.Exec(ctx, query, from, to)
	return err
}

func (r *directionRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM directions WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
