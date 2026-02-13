package postgres

import (
	"context"
	"fmt"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type orderRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewOrderRepo(db *pgxpool.Pool, log logger.ILogger) storage.IOrderStorage {
	return &orderRepo{db: db, log: log}
}

func (r *orderRepo) Create(ctx context.Context, order *models.Order) (*models.Order, error) {
	query := `
		INSERT INTO orders (client_id, driver_id, from_location_id, to_location_id, tariff_id, price, currency, passengers, pickup_time, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		order.ClientID,
		order.DriverID,
		order.FromLocationID,
		order.ToLocationID,
		order.TariffID,
		order.Price,
		order.Currency,
		order.Passengers,
		order.PickupTime,
		order.Status,
	).Scan(&order.ID, &order.CreatedAt)

	if err != nil {
		r.log.Error("failed to create order", logger.Error(err))
		return nil, err
	}

	return order, nil
}

func (r *orderRepo) Update(ctx context.Context, order *models.Order) (*models.Order, error) {
	query := `
		UPDATE orders
		SET driver_id = $1, status = $2, price = $3, passengers = $4, pickup_time = $5
		WHERE id = $6
		RETURNING created_at
	`
	err := r.db.QueryRow(ctx, query,
		order.DriverID,
		order.Status,
		order.Price,
		order.Passengers,
		order.PickupTime,
		order.ID,
	).Scan(&order.CreatedAt)

	if err != nil {
		r.log.Error("failed to update order", logger.Error(err))
		return nil, err
	}

	return order, nil
}

func (r *orderRepo) GetByID(ctx context.Context, id int64) (*models.Order, error) {
	var order models.Order
	query := `
		SELECT id, client_id, driver_id, from_location_id, to_location_id, tariff_id, price, currency, passengers, pickup_time, status, created_at
		FROM orders
		WHERE id = $1
	`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.ClientID,
		&order.DriverID,
		&order.FromLocationID,
		&order.ToLocationID,
		&order.TariffID,
		&order.Price,
		&order.Currency,
		&order.Passengers,
		&order.PickupTime,
		&order.Status,
		&order.CreatedAt,
	)

	if err != nil {
		r.log.Error("failed to get order by id", logger.Int64("id", id), logger.Error(err))
		return nil, err
	}

	return &order, nil
}

func (r *orderRepo) GetAll(ctx context.Context) ([]*models.Order, error) {
	query := `
		SELECT id, client_id, driver_id, from_location_id, to_location_id, tariff_id, price, currency, passengers, pickup_time, status, created_at
		FROM orders
		ORDER BY created_at DESC
	`
	return r.scanOrders(ctx, query)
}

func (r *orderRepo) GetClientOrders(ctx context.Context, clientID int64) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at,
		       COALESCE(fl.name, 'Noma''lum') as from_location_name,
		       COALESCE(tl.name, 'Noma''lum') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		WHERE o.client_id = $1
		ORDER BY o.created_at DESC
	`
	return r.scanOrders(ctx, query, clientID)
}

func (r *orderRepo) GetActiveOrders(ctx context.Context) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at,
		       COALESCE(fl.name, 'Noma''lum') as from_location_name,
		       COALESCE(tl.name, 'Noma''lum') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		WHERE o.status = 'active'
		ORDER BY o.created_at DESC
	`
	return r.scanOrders(ctx, query)
}

func (r *orderRepo) GetDriverOrders(ctx context.Context, driverID int64) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at,
		       COALESCE(fl.name, 'Noma''lum') as from_location_name,
		       COALESCE(tl.name, 'Noma''lum') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		WHERE o.driver_id = $1
		ORDER BY o.created_at DESC
	`
	return r.scanOrders(ctx, query, driverID)
}

func (r *orderRepo) scanOrders(ctx context.Context, query string, args ...interface{}) ([]*models.Order, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		var o models.Order
		err := rows.Scan(
			&o.ID, &o.ClientID, &o.DriverID, &o.FromLocationID, &o.ToLocationID, &o.TariffID,
			&o.Price, &o.Currency, &o.Passengers, &o.PickupTime, &o.Status, &o.CreatedAt,
			&o.FromLocationName, &o.ToLocationName,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &o)
	}
	return orders, nil
}

func (r *orderRepo) TakeOrder(ctx context.Context, orderID int64, driverID int64) error {
	res, err := r.db.Exec(ctx, "UPDATE orders SET status = 'taken', driver_id = $1 WHERE id = $2 AND status = 'active'", driverID, orderID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("buyurtma allaqachon olingan yoki bekor qilingan")
	}
	return nil
}

func (r *orderRepo) CompleteOrder(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'completed' WHERE id = $1 AND status = 'taken'", orderID)
	return err
}

func (r *orderRepo) CancelOrder(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'cancelled' WHERE id = $1 AND status = 'active'", orderID)
	return err
}
