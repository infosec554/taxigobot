package postgres

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type locationRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewLocationRepo(db *pgxpool.Pool, log logger.ILogger) storage.ILocationStorage {
	return &locationRepo{db: db, log: log}
}

func (r *locationRepo) GetAll(ctx context.Context) ([]*models.Location, error) {
	query := `SELECT id, name, created_at FROM locations ORDER BY id ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []*models.Location
	for rows.Next() {
		var l models.Location
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt); err != nil {
			return nil, err
		}
		locations = append(locations, &l)
	}
	return locations, nil
}

func (r *locationRepo) GetByID(ctx context.Context, id int64) (*models.Location, error) {
	var l models.Location
	query := `SELECT id, name, created_at FROM locations WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(&l.ID, &l.Name, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *locationRepo) Create(ctx context.Context, name string) error {
	query := `INSERT INTO locations (name) VALUES ($1)`
	_, err := r.db.Exec(ctx, query, name)
	return err
}

func (r *locationRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM locations WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
