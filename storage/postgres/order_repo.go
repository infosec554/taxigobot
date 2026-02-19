package postgres

import (
	"context"
	"fmt"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"
	"time"

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
		INSERT INTO orders (client_id, driver_id, from_location_id, to_location_id, tariff_id, price, currency, passengers, pickup_time, status, client_username, client_phone)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`

	// Handle nil pointer for driver_id
	var driverID interface{}
	if order.DriverID != nil {
		driverID = order.DriverID
	}

	// Handle empty strings for client fields
	clientUsername := order.ClientUsername
	if clientUsername == "" {
		clientUsername = "Неизвестно"
	}
	clientPhone := order.ClientPhone
	if clientPhone == "" {
		clientPhone = "Неизвестно"
	}

	err := r.db.QueryRow(ctx, query,
		order.ClientID,
		driverID,
		order.FromLocationID,
		order.ToLocationID,
		order.TariffID,
		order.Price,
		order.Currency,
		order.Passengers,
		order.PickupTime,
		order.Status,
		clientUsername,
		clientPhone,
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
		SELECT id, client_id, driver_id, from_location_id, to_location_id, tariff_id, price, currency, passengers, pickup_time, status, created_at, client_username, client_phone
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
		&order.ClientUsername,
		&order.ClientPhone,
	)

	if err != nil {
		r.log.Error("failed to get order by id", logger.Int64("id", id), logger.Error(err))
		return nil, err
	}

	return &order, nil
}

func (r *orderRepo) GetAll(ctx context.Context) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at, o.client_username, o.client_phone,
		       COALESCE(fl.name, 'Неизвестно') as from_location_name,
		       COALESCE(tl.name, 'Неизвестно') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		ORDER BY o.created_at DESC
	`
	return r.scanOrders(ctx, query)
}

func (r *orderRepo) GetClientOrders(ctx context.Context, clientID int64) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at, o.client_username, o.client_phone,
		       COALESCE(fl.name, 'Неизвестно') as from_location_name,
		       COALESCE(tl.name, 'Неизвестно') as to_location_name
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
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at, o.client_username, o.client_phone,
		       COALESCE(fl.name, 'Неизвестно') as from_location_name,
		       COALESCE(tl.name, 'Неизвестно') as to_location_name
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
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at, o.client_username, o.client_phone,
		       COALESCE(fl.name, 'Неизвестно') as from_location_name,
		       COALESCE(tl.name, 'Неизвестно') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		WHERE o.driver_id = $1
		ORDER BY o.created_at DESC
	`
	return r.scanOrders(ctx, query, driverID)
}

func (r *orderRepo) GetOrdersByDate(ctx context.Context, date time.Time, driverID int64) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at, o.client_username, o.client_phone,
		       COALESCE(fl.name, 'Неизвестно') as from_location_name,
		       COALESCE(tl.name, 'Неизвестно') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		JOIN driver_routes dr ON (o.from_location_id = dr.from_location_id AND o.to_location_id = dr.to_location_id)
		JOIN driver_tariffs dt ON (o.tariff_id = dt.tariff_id)
		WHERE o.status = 'active'
		  AND o.pickup_time::date = $1::date
		  AND dr.driver_id = $2
		  AND dt.driver_id = $2
		ORDER BY o.pickup_time ASC
	`
	return r.scanOrders(ctx, query, date, driverID)
}

func (r *orderRepo) RequestOrder(ctx context.Context, orderID int64, driverID int64) error {
	res, err := r.db.Exec(ctx, "UPDATE orders SET status = 'wait_confirm', driver_id = $1 WHERE id = $2 AND status = 'active'", driverID, orderID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("заказ уже обрабатывается, занят или отменен")
	}
	return nil
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
			&o.ClientUsername, &o.ClientPhone,
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
		return fmt.Errorf("заказ уже принят или отменен")
	}
	return nil
}

func (r *orderRepo) SetOrderOnWay(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'on_way', on_way_at = NOW() WHERE id = $1 AND status = 'taken'", orderID)
	return err
}

func (r *orderRepo) SetOrderArrived(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'arrived', arrived_at = NOW() WHERE id = $1 AND status = 'on_way'", orderID)
	return err
}

func (r *orderRepo) SetOrderInProgress(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'in_progress', started_at = NOW() WHERE id = $1 AND status = 'arrived'", orderID)
	return err
}

func (r *orderRepo) CompleteOrder(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'completed', completed_at = NOW() WHERE id = $1 AND status = 'in_progress'", orderID)
	return err
}

func (r *orderRepo) CancelOrder(ctx context.Context, orderID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = 'cancelled' WHERE id = $1 AND status IN ('pending', 'active', 'wait_confirm')", orderID)
	return err
}

func (r *orderRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET status = $1 WHERE id = $2", status, id)
	return err
}

func (r *orderRepo) GetPendingOrders(ctx context.Context) ([]*models.Order, error) {
	query := `
		SELECT o.id, o.client_id, o.driver_id, o.from_location_id, o.to_location_id, o.tariff_id, o.price, o.currency, o.passengers, o.pickup_time, o.status, o.created_at, o.client_username, o.client_phone,
		       COALESCE(fl.name, 'Неизвестно') as from_location_name,
		       COALESCE(tl.name, 'Неизвестно') as to_location_name
		FROM orders o
		LEFT JOIN locations fl ON o.from_location_id = fl.id
		LEFT JOIN locations tl ON o.to_location_id = tl.id
		WHERE o.status = 'pending'
		ORDER BY o.created_at ASC
	`
	return r.scanOrders(ctx, query)
}

func (r *orderRepo) GetActiveOrdersCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT count(*) FROM orders WHERE status = 'active' OR status = 'taken'").Scan(&count)
	return count, err
}

func (r *orderRepo) GetTotalOrdersCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&count)
	return count, err
}

func (r *orderRepo) GetClientStats(ctx context.Context, clientID int64) (total, completed, cancelled int, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'cancelled' OR status = 'cancelled_by_admin')
		FROM orders 
		WHERE client_id = $1
	`, clientID).Scan(&total, &completed, &cancelled)
	return
}

func (r *orderRepo) GetDailyOrderCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT count(*) FROM orders WHERE created_at >= CURRENT_DATE").Scan(&count)
	return count, err
}

func (r *orderRepo) GetGlobalCancelRate(ctx context.Context) (float64, error) {
	var total, cancelled int
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'cancelled' OR status = 'cancelled_by_admin')
		FROM orders
	`).Scan(&total, &cancelled)

	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	return float64(cancelled) / float64(total) * 100, nil
}
