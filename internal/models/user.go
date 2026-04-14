package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name,omitempty"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
