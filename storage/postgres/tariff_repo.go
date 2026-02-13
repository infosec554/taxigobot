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
	query := `SELECT id, name, created_at FROM tariffs ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tariffs []*models.Tariff
	for rows.Next() {
		var t models.Tariff
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tariffs = append(tariffs, &t)
	}
	return tariffs, nil
}

func (r *tariffRepo) GetByID(ctx context.Context, id int64) (*models.Tariff, error) {
	var t models.Tariff
	query := `SELECT id, name, created_at FROM tariffs WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(&t.ID, &t.Name, &t.CreatedAt)
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

func (r *tariffRepo) GetEnabled(ctx context.Context, driverID int64) (map[int64]bool, error) {
	query := `SELECT tariff_id FROM driver_tariffs WHERE driver_id = $1`
	rows, err := r.db.Query(ctx, query, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	enabled := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		enabled[id] = true
	}
	return enabled, nil
}

func (r *tariffRepo) Toggle(ctx context.Context, driverID, tariffID int64) (bool, error) {
	// Check if exists
	var exists bool
	queryCheck := `SELECT EXISTS(SELECT 1 FROM driver_tariffs WHERE driver_id = $1 AND tariff_id = $2)`
	err := r.db.QueryRow(ctx, queryCheck, driverID, tariffID).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		_, err = r.db.Exec(ctx, "DELETE FROM driver_tariffs WHERE driver_id = $1 AND tariff_id = $2", driverID, tariffID)
		return false, err
	} else {
		_, err = r.db.Exec(ctx, "INSERT INTO driver_tariffs (driver_id, tariff_id) VALUES ($1, $2)", driverID, tariffID)
		return true, err
	}
}

func (r *tariffRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM tariffs WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
