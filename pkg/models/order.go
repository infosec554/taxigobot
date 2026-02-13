package models

import "time"

type Order struct {
	ID             int64      `json:"id"`
	ClientID       int64      `json:"client_id"`
	DriverID       *int64     `json:"driver_id"`
	FromLocationID int64      `json:"from_location_id"`
	ToLocationID   int64      `json:"to_location_id"`
	TariffID       int64      `json:"tariff_id"`
	Price          int        `json:"price"`
	Currency       string     `json:"currency"`
	Passengers     int        `json:"passengers"`
	PickupTime     *time.Time `json:"pickup_time"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`

	// Client info for notifications
	ClientUsername string `json:"client_username"`
	ClientPhone    string `json:"client_phone"`

	// Joined fields
	FromLocationName string `json:"from_location_name"`
	ToLocationName   string `json:"to_location_name"`
}
