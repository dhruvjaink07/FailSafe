package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/google/uuid"
)

func GenerateAPIKey(environment, role, name string) (string, string) {
	entropy := make([]byte, 32)
	_, _ = rand.Read(entropy)

	parts := []string{"fs", normalizePart(environment), normalizePart(role)}
	if normalizedName := normalizePart(name); normalizedName != "" {
		parts = append(parts, normalizedName)
	}
	parts = append(parts, hex.EncodeToString(entropy))

	raw := strings.Join(parts, "_")
	hash := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(hash[:])
}

func normalizePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, value)
	return strings.Trim(value, "-_")
}

func (p *Postgres) CreateAPIKey(environment, role, name, ownerID string) (string, error) {
	raw, hashHex := GenerateAPIKey(environment, role, name)
	keyID := uuid.New()

	if strings.TrimSpace(ownerID) == "" {
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO api_keys (id, key_hash, environment, role)
			VALUES ($1, $2, $3, $4)
		`, keyID, hashHex, strings.TrimSpace(environment), strings.TrimSpace(role))
		if err != nil {
			return "", err
		}
	} else {
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO api_keys (id, key_hash, environment, role, owner_id)
			VALUES ($1, $2, $3, $4, $5)
		`, keyID, hashHex, strings.TrimSpace(environment), strings.TrimSpace(role), ownerID)
		if err != nil {
			return "", err
		}
	}

	return raw, nil
}

func (p *Postgres) LookupAPIKeyByHash(hashHex string) (*models.APIKey, error) {
	var apiKey models.APIKey
	err := p.Pool.QueryRow(context.Background(), `
		SELECT id, key_hash, environment, role, created_at
		FROM api_keys
		WHERE key_hash = $1
	`, strings.TrimSpace(hashHex)).Scan(
		&apiKey.ID,
		&apiKey.KeyHash,
		&apiKey.Environment,
		&apiKey.Role,
		&apiKey.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (p *Postgres) CountAPIKeys() (int, error) {
	var count int
	err := p.Pool.QueryRow(context.Background(), `SELECT COUNT(1) FROM api_keys`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (p *Postgres) ListAPIKeys(environment string) ([]models.APIKey, error) {
	var rows pgx.Rows
	var err error
	if strings.TrimSpace(environment) == "" {
		rows, err = p.Pool.Query(context.Background(), `
			SELECT id, key_hash, environment, role, created_at FROM api_keys ORDER BY created_at DESC
		`)
	} else {
		rows, err = p.Pool.Query(context.Background(), `
			SELECT id, key_hash, environment, role, created_at FROM api_keys WHERE environment = $1 ORDER BY created_at DESC
		`, strings.TrimSpace(environment))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.APIKey, 0)
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.KeyHash, &k.Environment, &k.Role, &k.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return result, nil
}

func (p *Postgres) PingAPIKeyStore() error {
	if p == nil || p.Pool == nil {
		return fmt.Errorf("postgres not initialized")
	}
	return nil
}

// RevokeAPIKey deletes an API key by id
func (p *Postgres) RevokeAPIKey(id string) error {
	_, err := p.Pool.Exec(context.Background(), `DELETE FROM api_keys WHERE id = $1`, id)
	return err
}

// RotateAPIKey generates a new raw key and replaces the stored hash for the given id
func (p *Postgres) RotateAPIKey(id string) (string, error) {
	// We need to generate a new raw key; use empty environment/role/name placeholders since GenerateAPIKey needs them.
	// First fetch existing env and role to preserve naming parts
	var env, role, name string
	err := p.Pool.QueryRow(context.Background(), `SELECT environment, role, '' FROM api_keys WHERE id = $1`, id).Scan(&env, &role, &name)
	if err != nil {
		return "", err
	}
	raw, hashHex := GenerateAPIKey(env, role, name)
	_, err = p.Pool.Exec(context.Background(), `UPDATE api_keys SET key_hash = $1 WHERE id = $2`, hashHex, id)
	if err != nil {
		return "", err
	}
	return raw, nil
}
