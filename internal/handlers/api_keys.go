package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

type APIKeyStore interface {
	CreateAPIKey(environment, role, name string) (string, error)
	LookupAPIKeyByHash(hashHex string) (*models.APIKey, error)
}

const apiContextKey = "api_ctx"

var roleRank = map[string]int{
	"viewer":   1,
	"engineer": 2,
	"admin":    3,
}

type createAPIKeyRequest struct {
	Environment string `json:"environment"`
	Role        string `json:"role"`
	Name        string `json:"name"`
}

func APIContextFromRequest(r *http.Request) (models.APIContext, bool) {
	ctx, ok := r.Context().Value(apiContextKey).(models.APIContext)
	return ctx, ok
}

func RequireAPIKey(store APIKeyStore, allowedRoles []string, action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("x-api-key"))
		if key == "" {
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}

		hash := sha256.Sum256([]byte(key))
		apiKey, err := store.LookupAPIKeyByHash(hex.EncodeToString(hash[:]))
		if err != nil || apiKey == nil {
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		if !roleAllowed(apiKey.Role, allowedRoles) {
			http.Error(w, "not allowed", http.StatusForbidden)
			return
		}

		apiCtx := models.APIContext{
			KeyID: apiKey.ID,
			Env:   apiKey.Environment,
			Role:  apiKey.Role,
		}
		log.Printf("api_key_id=%s endpoint=%s timestamp=%s action=%s", apiCtx.KeyID, r.URL.Path, time.Now().UTC().Format(time.RFC3339), action)

		requestWithCtx := r.WithContext(context.WithValue(r.Context(), apiContextKey, apiCtx))
		next(w, requestWithCtx)
	}
}

func CreateAPIKeyHandler(store APIKeyStore, bootstrapToken string) http.HandlerFunc {
	_ = bootstrapToken
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req createAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		env := normalizeAPIKeyEnvironment(req.Environment)
		role := normalizeAPIKeyRole(req.Role)
		if env == "" || role == "" {
			http.Error(w, "environment and role are required", http.StatusBadRequest)
			return
		}

		raw, err := store.CreateAPIKey(env, role, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("api_key_id=public endpoint=%s timestamp=%s action=create_api_key env=%s role=%s", r.URL.Path, time.Now().UTC().Format(time.RFC3339), env, role)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"api_key": raw})
	}
}

func normalizeAPIKeyEnvironment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dev", "prod":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeAPIKeyRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "viewer", "engineer", "admin":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func roleAllowed(role string, allowedRoles []string) bool {
	if len(allowedRoles) == 0 {
		return true
	}
	current, ok := roleRank[strings.ToLower(strings.TrimSpace(role))]
	if !ok {
		return false
	}
	minRank := 0
	for _, allowed := range allowedRoles {
		rank, ok := roleRank[strings.ToLower(strings.TrimSpace(allowed))]
		if !ok {
			continue
		}
		if minRank == 0 || rank < minRank {
			minRank = rank
		}
	}
	return current >= minRank && minRank > 0
}
