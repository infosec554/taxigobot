package service

import (
	"taxibot/pkg/logger"
	"taxibot/storage"
)

type IServiceManager interface {
	User() UserService
	Order() OrderService
}

type service struct {
	userService  UserService
	orderService OrderService
}

func New(stg storage.IStorage, log logger.ILogger) IServiceManager {
	return &service{
		userService:  NewUserService(stg, log),
		orderService: NewOrderService(stg, log),
	}
}

func (s *service) User() UserService {
	return s.userService
}

func (s *service) Order() OrderService {
	return s.orderService
}
