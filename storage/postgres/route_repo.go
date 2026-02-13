package postgres

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type routeRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewRouteRepo(db *pgxpool.Pool, log logger.ILogger) storage.IRouteStorage {
	return &routeRepo{db: db, log: log}
}

func (r *routeRepo) AddRoute(ctx context.Context, driverID, fromLocationID, toLocationID int64) error {
	query := `INSERT INTO driver_routes (driver_id, from_location_id, to_location_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, driverID, fromLocationID, toLocationID)
	return err
}

func (r *routeRepo) RemoveRoute(ctx context.Context, driverID, fromLocationID, toLocationID int64) error {
	query := `DELETE FROM driver_routes WHERE driver_id = $1 AND from_location_id = $2 AND to_location_id = $3`
	_, err := r.db.Exec(ctx, query, driverID, fromLocationID, toLocationID)
	return err
}

func (r *routeRepo) GetDriverRoutes(ctx context.Context, driverID int64) ([][2]int64, error) {
	query := `SELECT from_location_id, to_location_id FROM driver_routes WHERE driver_id = $1`
	rows, err := r.db.Query(ctx, query, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes [][2]int64
	for rows.Next() {
		var fromID, toID int64
		if err := rows.Scan(&fromID, &toID); err != nil {
			return nil, err
		}
		routes = append(routes, [2]int64{fromID, toID})
	}
	return routes, nil
}

func (r *routeRepo) GetDriversByRoute(ctx context.Context, fromLocationID, toLocationID int64) ([]int64, error) {
	query := `SELECT driver_id FROM driver_routes WHERE from_location_id = $1 AND to_location_id = $2`
	rows, err := r.db.Query(ctx, query, fromLocationID, toLocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var driverIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		driverIDs = append(driverIDs, id)
	}
	return driverIDs, nil
}

func (r *routeRepo) ClearRoutes(ctx context.Context, driverID int64) error {
	query := `DELETE FROM driver_routes WHERE driver_id = $1`
	_, err := r.db.Exec(ctx, query, driverID)
	return err
}
