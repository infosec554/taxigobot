package models

type CarBrand struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CarModel struct {
	ID      int64  `json:"id"`
	BrandID int64  `json:"brand_id"`
	Name    string `json:"name"`
}

type DriverProfile struct {
	UserID       int64  `json:"user_id"`
	CarBrand     string `json:"car_brand"`
	CarModel     string `json:"car_model"`
	LicensePlate string `json:"license_plate"`
	Status       string `json:"status"` // pending_review, active, rejected, blocked
}
