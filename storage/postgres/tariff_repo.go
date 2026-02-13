package postgres

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type tariffRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewTariffRepo(db *pgxpool.Pool, log logger.ILogger) storage.ITariffStorage {
	return &tariffRepo{db: db, log: log}
}

func (r *tariffRepo) GetAll(ctx context.Context) ([]*models.Tariff, error) {
	query := `SELECT id, name, is_active, created_at FROM tariffs ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tariffs []*models.Tariff
	for rows.Next() {
		var t models.Tariff
		if err := rows.Scan(&t.ID, &t.Name, &t.IsActive, &t.CreatedAt); err != nil {
			return nil, err
		}
		tariffs = append(tariffs, &t)
	}
	return tariffs, nil
}

func (r *tariffRepo) GetByID(ctx context.Context, id int64) (*models.Tariff, error) {
	var t models.Tariff
	query := `SELECT id, name, is_active, created_at FROM tariffs WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(&t.ID, &t.Name, &t.IsActive, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tariffRepo) Create(ctx context.Context, name string) error {
	query := `INSERT INTO tariffs (name) VALUES ($1)`
	_, err := r.db.Exec(ctx, query, name)
	return err
}

func (r *tariffRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM tariffs WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
