package storage

import (
	"context"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) CreateUser(email, name, passwordHash, role string) (string, error) {
	id := uuid.New().String()
	_, err := p.Pool.Exec(context.Background(), `
        INSERT INTO users (id, email, name, password_hash, role, created_at)
        VALUES ($1,$2,$3,$4,$5,$6)
    `, id, email, name, passwordHash, role, time.Now())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (p *Postgres) GetUserByEmail(email string) (*models.User, error) {
	var u models.User
	err := p.Pool.QueryRow(context.Background(), `
        SELECT id, email, name, password_hash, role, created_at FROM users WHERE email=$1
    `, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (p *Postgres) GetUserByID(id string) (*models.User, error) {
	var u models.User
	err := p.Pool.QueryRow(context.Background(), `
        SELECT id, email, name, password_hash, role, created_at FROM users WHERE id=$1
    `, id).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
