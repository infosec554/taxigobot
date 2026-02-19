package postgres

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type carRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewCarRepo(db *pgxpool.Pool, log logger.ILogger) storage.ICarStorage {
	return &carRepo{db: db, log: log}
}

func (r *carRepo) GetBrands(ctx context.Context) ([]*models.CarBrand, error) {
	query := "SELECT id, name FROM car_brands ORDER BY name"
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.log.Error("failed to get car brands", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	var brands []*models.CarBrand
	for rows.Next() {
		var b models.CarBrand
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		brands = append(brands, &b)
	}
	return brands, nil
}

func (r *carRepo) GetModels(ctx context.Context, brandID int64) ([]*models.CarModel, error) {
	query := "SELECT id, brand_id, name FROM car_models WHERE brand_id = $1 ORDER BY name"
	rows, err := r.db.Query(ctx, query, brandID)
	if err != nil {
		r.log.Error("failed to get car models", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	var modelsList []*models.CarModel
	for rows.Next() {
		var m models.CarModel
		if err := rows.Scan(&m.ID, &m.BrandID, &m.Name); err != nil {
			return nil, err
		}
		modelsList = append(modelsList, &m)
	}
	return modelsList, nil
}

func (r *carRepo) CreateBrand(ctx context.Context, name string) error {
	query := `INSERT INTO car_brands (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`
	_, err := r.db.Exec(ctx, query, name)
	return err
}

func (r *carRepo) CreateModel(ctx context.Context, brandID int64, name string) error {
	query := `INSERT INTO car_models (brand_id, name) VALUES ($1, $2)`
	_, err := r.db.Exec(ctx, query, brandID, name)
	return err
}

func (r *carRepo) DeleteBrand(ctx context.Context, id int64) error {
	// car_models has ON DELETE CASCADE so models are auto-deleted
	_, err := r.db.Exec(ctx, `DELETE FROM car_brands WHERE id = $1`, id)
	return err
}

func (r *carRepo) DeleteModel(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM car_models WHERE id = $1`, id)
	return err
}
