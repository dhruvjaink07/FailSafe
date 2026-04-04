package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

type fakeAPIKeyStore struct {
	keys    map[string]*models.APIKey
	created struct {
		environment string
		role        string
		name        string
	}
}

func (f *fakeAPIKeyStore) CreateAPIKey(environment, role, name string) (string, error) {
	f.created.environment = environment
	f.created.role = role
	f.created.name = name
	return "fs_test_key", nil
}

func (f *fakeAPIKeyStore) LookupAPIKeyByHash(hashHex string) (*models.APIKey, error) {
	if apiKey, ok := f.keys[hashHex]; ok {
		return apiKey, nil
	}
	return nil, fmt.Errorf("not found")
}

func TestRequireAPIKeyEnforcesAccess(t *testing.T) {
	viewerRaw := "fs_viewer"
	engineerRaw := "fs_engineer"

	viewerHash := sha256.Sum256([]byte(viewerRaw))
	engineerHash := sha256.Sum256([]byte(engineerRaw))

	store := &fakeAPIKeyStore{keys: map[string]*models.APIKey{
		hex.EncodeToString(viewerHash[:]): {
			ID:          "viewer-id",
			KeyHash:     hex.EncodeToString(viewerHash[:]),
			Environment: "dev",
			Role:        "viewer",
			CreatedAt:   time.Now(),
		},
		hex.EncodeToString(engineerHash[:]): {
			ID:          "engineer-id",
			KeyHash:     hex.EncodeToString(engineerHash[:]),
			Environment: "prod",
			Role:        "engineer",
			CreatedAt:   time.Now(),
		},
	}}

	protected := RequireAPIKey(store, []string{"engineer", "admin"}, "start_experiment", func(w http.ResponseWriter, r *http.Request) {
		ctx, ok := APIContextFromRequest(r)
		if !ok {
			t.Fatal("expected api context")
		}
		if ctx.KeyID != "engineer-id" {
			t.Fatalf("unexpected key id %s", ctx.KeyID)
		}
		w.WriteHeader(http.StatusOK)
	})

	missingReq := httptest.NewRequest(http.MethodPost, "/experiments/frontend/start", nil)
	missingRes := httptest.NewRecorder()
	protected(missingRes, missingReq)
	if missingRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing key, got %d", missingRes.Code)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/experiments/frontend/start", nil)
	invalidReq.Header.Set("x-api-key", "fs_invalid")
	invalidRes := httptest.NewRecorder()
	protected(invalidRes, invalidReq)
	if invalidRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid key, got %d", invalidRes.Code)
	}

	viewerReq := httptest.NewRequest(http.MethodPost, "/experiments/frontend/start", nil)
	viewerReq.Header.Set("x-api-key", viewerRaw)
	viewerRes := httptest.NewRecorder()
	protected(viewerRes, viewerReq)
	if viewerRes.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer, got %d", viewerRes.Code)
	}

	engineerReq := httptest.NewRequest(http.MethodPost, "/experiments/frontend/start", nil)
	engineerReq.Header.Set("x-api-key", engineerRaw)
	engineerRes := httptest.NewRecorder()
	protected(engineerRes, engineerReq)
	if engineerRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for engineer, got %d", engineerRes.Code)
	}
}

func TestCreateAPIKeyHandler(t *testing.T) {
	store := &fakeAPIKeyStore{}

	handler := CreateAPIKeyHandler(store, "bootstrap-secret")

	req := httptest.NewRequest(http.MethodPost, "/internal/api-keys/create", strings.NewReader(`{"environment":"dev","role":"engineer","name":"team-a"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["api_key"] != "fs_test_key" {
		t.Fatalf("unexpected api key response: %v", body)
	}
	if store.created.environment != "dev" || store.created.role != "engineer" || store.created.name != "team-a" {
		t.Fatalf("unexpected create params: %+v", store.created)
	}
}

func TestCreateAPIKeyHandlerAllowsAnySupportedRoleAndEnv(t *testing.T) {
	store := &fakeAPIKeyStore{}
	handler := CreateAPIKeyHandler(store, "bootstrap-secret")

	prodReq := httptest.NewRequest(http.MethodPost, "/internal/api-keys/create", strings.NewReader(`{"environment":"prod","role":"engineer","name":"team-a"}`))
	prodReq.Header.Set("Content-Type", "application/json")
	prodRes := httptest.NewRecorder()
	handler(prodRes, prodReq)
	if prodRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for creating prod key, got %d", prodRes.Code)
	}

	adminReq := httptest.NewRequest(http.MethodPost, "/internal/api-keys/create", strings.NewReader(`{"environment":"dev","role":"admin","name":"team-a"}`))
	adminReq.Header.Set("Content-Type", "application/json")
	adminRes := httptest.NewRecorder()
	handler(adminRes, adminReq)
	if adminRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for creating admin key, got %d", adminRes.Code)
	}
}
