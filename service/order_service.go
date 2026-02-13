package service

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"
)

type OrderService interface {
	CreateOrder(ctx context.Context, order *models.Order) (*models.Order, error)
	UpdateOrder(ctx context.Context, order *models.Order) (*models.Order, error)
	GetByID(ctx context.Context, id int64) (*models.Order, error)
}

type orderService struct {
	stg storage.IOrderStorage
	log logger.ILogger
}

func NewOrderService(stg storage.IStorage, log logger.ILogger) OrderService {
	return &orderService{
		stg: stg.Order(),
		log: log,
	}
}

func (s *orderService) CreateOrder(ctx context.Context, order *models.Order) (*models.Order, error) {
	return s.stg.Create(ctx, order)
}

func (s *orderService) UpdateOrder(ctx context.Context, order *models.Order) (*models.Order, error) {
	return s.stg.Update(ctx, order)
}

func (s *orderService) GetByID(ctx context.Context, id int64) (*models.Order, error) {
	return s.stg.GetByID(ctx, id)
}
