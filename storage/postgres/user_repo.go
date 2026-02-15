package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"
)

type userRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewUserRepo(db *pgxpool.Pool, log logger.ILogger) storage.IUserStorage {
	return &userRepo{db: db, log: log}
}

func (r *userRepo) GetOrCreate(ctx context.Context, teleID int64, username, fullname string) (*models.User, error) {
	var user models.User
	query := `
		INSERT INTO users (telegram_id, username, full_name, role, status)
		VALUES ($1, $2, $3, 'client', 'pending')
		ON CONFLICT (telegram_id) DO UPDATE 
		SET updated_at = NOW()
		RETURNING id, telegram_id, full_name, username, phone, role, status, language, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, teleID, username, fullname).Scan(
		&user.ID, &user.TelegramID, &user.FullName, &user.Username, &user.Phone, &user.Role, &user.Status, &user.Language, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		r.log.Error("failed to get or create user", logger.Error(err))
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	query := `SELECT id, telegram_id, full_name, username, phone, role, status, language, created_at, updated_at FROM users WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.TelegramID, &user.FullName, &user.Username, &user.Phone, &user.Role, &user.Status, &user.Language, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		r.log.Error("failed to get user by id", logger.Error(err))
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) Get(ctx context.Context, teleID int64) (*models.User, error) {
	var user models.User
	query := `SELECT id, telegram_id, full_name, username, phone, role, status, language, created_at, updated_at FROM users WHERE telegram_id = $1`
	err := r.db.QueryRow(ctx, query, teleID).Scan(
		&user.ID, &user.TelegramID, &user.FullName, &user.Username, &user.Phone, &user.Role, &user.Status, &user.Language, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		r.log.Error("failed to get user", logger.Error(err))
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) GetAll(ctx context.Context) ([]*models.User, error) {
	query := `SELECT id, telegram_id, full_name, username, phone, role, status, language, created_at, updated_at FROM users`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(
			&u.ID, &u.TelegramID, &u.FullName, &u.Username, &u.Phone, &u.Role, &u.Status, &u.Language, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (r *userRepo) UpdateLanguage(ctx context.Context, teleID int64, lang string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET language=$1 WHERE telegram_id=$2", lang, teleID)
	return err
}

func (r *userRepo) UpdateStatus(ctx context.Context, teleID int64, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET status=$1 WHERE telegram_id=$2", status, teleID)
	return err
}

func (r *userRepo) UpdateStatusByID(ctx context.Context, id int64, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET status=$1 WHERE id=$2", status, id)
	return err
}

func (r *userRepo) UpdateRole(ctx context.Context, teleID int64, role string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET role=$1 WHERE telegram_id=$2", role, teleID)
	return err
}

func (r *userRepo) UpdateRoleByID(ctx context.Context, id int64, role string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET role=$1 WHERE id=$2", role, id)
	return err
}

func (r *userRepo) UpdatePhone(ctx context.Context, teleID int64, phone string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET phone=$1, status='active' WHERE telegram_id=$2", phone, teleID)
	return err
}

func (r *userRepo) GetPendingDrivers(ctx context.Context) ([]*models.User, error) {
	query := `SELECT id, telegram_id, full_name, username, phone, role, status, language, created_at, updated_at FROM users WHERE role = 'driver' AND (status = 'pending' OR status = 'pending_review')`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(
			&u.ID, &u.TelegramID, &u.FullName, &u.Username, &u.Phone, &u.Role, &u.Status, &u.Language, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (r *userRepo) GetBlockedUsers(ctx context.Context) ([]*models.User, error) {
	query := `SELECT id, telegram_id, full_name, username, phone, role, status, language, created_at, updated_at FROM users WHERE status = 'blocked' ORDER BY updated_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(
			&u.ID, &u.TelegramID, &u.FullName, &u.Username, &u.Phone, &u.Role, &u.Status, &u.Language, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (r *userRepo) GetTotalUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count)
	return count, err
}

func (r *userRepo) GetTotalDrivers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT count(*) FROM users WHERE role = 'driver'").Scan(&count)
	return count, err
}

func (r *userRepo) CreateDriverProfile(ctx context.Context, profile *models.DriverProfile) error {
	query := `
		INSERT INTO driver_profiles (user_id, car_brand, car_model, license_plate)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE 
		SET car_brand = EXCLUDED.car_brand,
			car_model = EXCLUDED.car_model,
			license_plate = EXCLUDED.license_plate
	`
	_, err := r.db.Exec(ctx, query, profile.UserID, profile.CarBrand, profile.CarModel, profile.LicensePlate)
	if err != nil {
		r.log.Error("failed to create driver profile", logger.Error(err))
		return err
	}
	return nil
}

func (r *userRepo) GetDriverProfile(ctx context.Context, userID int64) (*models.DriverProfile, error) {
	var profile models.DriverProfile
	query := `SELECT user_id, car_brand, car_model, license_plate FROM driver_profiles WHERE user_id = $1`
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&profile.UserID, &profile.CarBrand, &profile.CarModel, &profile.LicensePlate,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		r.log.Error("failed to get driver profile", logger.Error(err))
		return nil, err
	}
	// Enrich Status from users table for easier access if needed, but not strictly required by struct yet
	// Models.DriverProfile has Status field now, let's fetch it from users table join

	// Actually better to do a JOIN to get status in one go if I want to populate Status field
	// But let's keep it simple or do a join

	var status string
	err = r.db.QueryRow(ctx, "SELECT status FROM users WHERE id=$1", userID).Scan(&status)
	if err == nil {
		profile.Status = status
	}

	return &profile, nil
}
