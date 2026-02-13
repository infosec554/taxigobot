package models

import "time"

type Order struct {
	ID          int64      `json:"id"`
	ClientID    int64      `json:"client_id"`
	DriverID    *int64     `json:"driver_id"`
	DirectionID int64      `json:"direction_id"`
	TariffID    int64      `json:"tariff_id"`
	Price       int        `json:"price"`
	Currency    string     `json:"currency"`
	Passengers  int        `json:"passengers"`
	PickupTime  *time.Time `json:"pickup_time"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
}
