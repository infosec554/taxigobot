package storage

import (
	"context"
	"taxibot/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IStorage interface {
	User() IUserStorage
	Order() IOrderStorage
	Tariff() ITariffStorage
	Location() ILocationStorage
	Route() IRouteStorage
	Close()
	GetPool() *pgxpool.Pool
}

type IUserStorage interface {
	GetOrCreate(ctx context.Context, teleID int64, username, fullname string) (*models.User, error)
	Get(ctx context.Context, teleID int64) (*models.User, error)
	GetAll(ctx context.Context) ([]*models.User, error)
	UpdateLanguage(ctx context.Context, teleID int64, lang string) error
	UpdateStatus(ctx context.Context, teleID int64, status string) error
	UpdateRole(ctx context.Context, teleID int64, role string) error
	UpdatePhone(ctx context.Context, teleID int64, phone string) error
}

type IOrderStorage interface {
	Create(ctx context.Context, order *models.Order) (*models.Order, error)
	Update(ctx context.Context, order *models.Order) (*models.Order, error)
	GetByID(ctx context.Context, id int64) (*models.Order, error)
	GetAll(ctx context.Context) ([]*models.Order, error)
	GetClientOrders(ctx context.Context, clientID int64) ([]*models.Order, error)
	GetActiveOrders(ctx context.Context) ([]*models.Order, error)
	GetDriverOrders(ctx context.Context, driverID int64) ([]*models.Order, error)
	TakeOrder(ctx context.Context, orderID int64, driverID int64) error
	CompleteOrder(ctx context.Context, orderID int64) error
	CancelOrder(ctx context.Context, orderID int64) error
}

type ITariffStorage interface {
	GetAll(ctx context.Context) ([]*models.Tariff, error)
	GetByID(ctx context.Context, id int64) (*models.Tariff, error)
	GetEnabled(ctx context.Context, driverID int64) (map[int64]bool, error)
	Toggle(ctx context.Context, driverID, tariffID int64) (bool, error)
	Create(ctx context.Context, name string) error
	Delete(ctx context.Context, id int64) error
}

type ILocationStorage interface {
	GetAll(ctx context.Context) ([]*models.Location, error)
	GetByID(ctx context.Context, id int64) (*models.Location, error)
	Create(ctx context.Context, name string) error
	Delete(ctx context.Context, id int64) error
}

type IRouteStorage interface {
	AddRoute(ctx context.Context, driverID, fromLocationID, toLocationID int64) error
	RemoveRoute(ctx context.Context, driverID, fromLocationID, toLocationID int64) error
	GetDriverRoutes(ctx context.Context, driverID int64) ([][2]int64, error)
	GetDriversByRoute(ctx context.Context, fromLocationID, toLocationID int64) ([]int64, error)
	ClearRoutes(ctx context.Context, driverID int64) error
}
