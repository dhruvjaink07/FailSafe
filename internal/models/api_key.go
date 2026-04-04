package models

import "time"

type APIKey struct {
	ID          string    `json:"id"`
	KeyHash     string    `json:"key_hash"`
	Environment string    `json:"environment"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}
