package models

import "time"

type Direction struct {
	ID           int64     `json:"id"`
	FromLocation string    `json:"from_location"`
	ToLocation   string    `json:"to_location"`
	CreatedAt    time.Time `json:"created_at"`
}
