package service

import (
	"context"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"
)

type UserService interface {
	Register(ctx context.Context, teleID int64, username, fullname string) (*models.User, error)
	Get(ctx context.Context, teleID int64) (*models.User, error)
	SetLanguage(ctx context.Context, teleID int64, lang string) error
	SetStatus(ctx context.Context, teleID int64, status string) error
	SetRole(ctx context.Context, teleID int64, role string) error
	SetPhone(ctx context.Context, teleID int64, phone string) error
}

type userService struct {
	stg storage.IUserStorage
	log logger.ILogger
}

func NewUserService(stg storage.IStorage, log logger.ILogger) UserService {
	return &userService{
		stg: stg.User(),
		log: log,
	}
}

func (s *userService) Register(ctx context.Context, teleID int64, username, fullname string) (*models.User, error) {
	return s.stg.GetOrCreate(ctx, teleID, username, fullname)
}

func (s *userService) Get(ctx context.Context, teleID int64) (*models.User, error) {
	return s.stg.Get(ctx, teleID)
}

func (s *userService) SetLanguage(ctx context.Context, teleID int64, lang string) error {
	return s.stg.UpdateLanguage(ctx, teleID, lang)
}

func (s *userService) SetStatus(ctx context.Context, teleID int64, status string) error {
	return s.stg.UpdateStatus(ctx, teleID, status)
}

func (s *userService) SetRole(ctx context.Context, teleID int64, role string) error {
	return s.stg.UpdateRole(ctx, teleID, role)
}

func (s *userService) SetPhone(ctx context.Context, teleID int64, phone string) error {
	return s.stg.UpdatePhone(ctx, teleID, phone)
}
