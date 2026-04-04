package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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

func (p *Postgres) CreateAPIKey(environment, role, name string) (string, error) {
	raw, hashHex := GenerateAPIKey(environment, role, name)
	keyID := uuid.New()

	_, err := p.Pool.Exec(context.Background(), `
		INSERT INTO api_keys (id, key_hash, environment, role)
		VALUES ($1, $2, $3, $4)
	`, keyID, hashHex, strings.TrimSpace(environment), strings.TrimSpace(role))
	if err != nil {
		return "", err
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

func (p *Postgres) PingAPIKeyStore() error {
	if p == nil || p.Pool == nil {
		return fmt.Errorf("postgres not initialized")
	}
	return nil
}
