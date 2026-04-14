package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhruvjaink07/failsafe/internal/models"
)

type fakeKeyStore struct {
	revokedID string
	rotatedID string
	rotateKey string
}

func (f *fakeKeyStore) CreateAPIKey(environment, role, name, ownerID string) (string, error) {
	return "fake", nil
}
func (f *fakeKeyStore) LookupAPIKeyByHash(hashHex string) (*models.APIKey, error) { return nil, nil }
func (f *fakeKeyStore) ListAPIKeys(environment string) ([]models.APIKey, error)   { return nil, nil }
func (f *fakeKeyStore) RevokeAPIKey(id string) error                              { f.revokedID = id; return nil }
func (f *fakeKeyStore) RotateAPIKey(id string) (string, error) {
	f.rotatedID = id
	return f.rotateKey, nil
}

func TestRevokeAPIKeyHandler(t *testing.T) {
	store := &fakeKeyStore{}
	handler := RevokeAPIKeyHandler(store)

	body, _ := json.Marshal(map[string]string{"id": "abc-123"})
	req := httptest.NewRequest(http.MethodPost, "/internal/api-keys/revoke", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)
	res := w.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.StatusCode)
	}
	if store.revokedID != "abc-123" {
		t.Fatalf("expected revoked id recorded, got %s", store.revokedID)
	}
}

func TestRotateAPIKeyHandler(t *testing.T) {
	store := &fakeKeyStore{rotateKey: "new-raw-key"}
	handler := RotateAPIKeyHandler(store)

	body, _ := json.Marshal(map[string]string{"id": "rotate-1"})
	req := httptest.NewRequest(http.MethodPost, "/internal/api-keys/rotate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)
	res := w.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	b, _ := io.ReadAll(res.Body)
	var out map[string]string
	_ = json.Unmarshal(b, &out)
	if out["api_key"] != "new-raw-key" {
		t.Fatalf("unexpected api_key value: %v", out)
	}
	if store.rotatedID != "rotate-1" {
		t.Fatalf("rotate not invoked properly: %s", store.rotatedID)
	}
}
