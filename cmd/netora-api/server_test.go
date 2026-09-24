package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/configengine"
	"netora-dns/pkg/model"
	"netora-dns/pkg/storage"
)

func setupTestServer(t *testing.T) (*Server, *storage.MemoryStore, string) {
	tempDir, err := os.MkdirTemp("", "netora-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store := storage.NewMemoryStore()
	hash, _ := auth.HashPassword("admin123")
	user := &auth.User{
		ID:           "usr-test",
		Username:     "testadmin",
		PasswordHash: hash,
		Role:         auth.RoleAdmin,
		FullName:     "Test Admin",
	}
	_ = store.CreateUser(context.Background(), user)

	gen, _ := configengine.NewGenerator()
	engine, _ := configengine.NewEngine(configengine.EngineOptions{
		ConfigDir:       tempDir,
		Generator:       gen,
		SkipHealthCheck: true,
	})

	jwtSecret := "test-secret-signing-key-123456789"
	server := NewServer(store, engine, jwtSecret, "test-node")

	token, _ := auth.GenerateToken(user, jwtSecret, 1*time.Hour)
	return server, store, token
}

func TestAPI_LoginSuccess(t *testing.T) {
	server, _, _ := setupTestServer(t)

	body := []byte(`{"username":"testadmin","password":"admin123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed parsing login response: %v", err)
	}
	if resp["token"] == "" {
		t.Errorf("expected non-empty token")
	}
}

func TestAPI_LoginInvalidCredentials(t *testing.T) {
	server, _, _ := setupTestServer(t)

	body := []byte(`{"username":"testadmin","password":"wrongpassword"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAPI_ProtectedEndpointWithoutAuth(t *testing.T) {
	server, _, _ := setupTestServer(t)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for unauthorized access, got %d", w.Code)
	}
}

func TestAPI_ProtectedEndpointWithValidAuth(t *testing.T) {
	server, store, token := setupTestServer(t)

	// Add a dummy node
	_ = store.UpsertNode(context.Background(), &model.NodeMetadata{
		NodeID:     "node-test-1",
		Hostname:   "dns.test.local",
		CPUCores:   2,
		TotalRAMMB: 2048,
	}, "HEALTHY", "10.0.0.1")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var nodes []storage.NodeRecord
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("failed decoding nodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "node-test-1" {
		t.Errorf("expected 1 node with ID node-test-1, got %+v", nodes)
	}
}

func TestAPI_AllowlistOperations(t *testing.T) {
	server, _, token := setupTestServer(t)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// 1. Add to allowlist
	addBody := []byte(`{"domain":"internal.corp","category":"INTERNAL","reason":"Internal ERP"}`)
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/allowlist", bytes.NewReader(addBody))
	addReq.Header.Set("Authorization", "Bearer "+token)
	addW := httptest.NewRecorder()
	mux.ServeHTTP(addW, addReq)

	if addW.Code != http.StatusOK {
		t.Fatalf("expected 200 adding allowlist, got %d", addW.Code)
	}

	// 2. List allowlist
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/allowlist", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listW := httptest.NewRecorder()
	mux.ServeHTTP(listW, listReq)

	var list []storage.AllowlistRecord
	_ = json.Unmarshal(listW.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Domain != "internal.corp" {
		t.Errorf("expected internal.corp in allowlist, got %+v", list)
	}

	// 3. Delete from allowlist
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/allowlist/internal.corp", nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delW := httptest.NewRecorder()
	mux.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Errorf("expected 200 deleting allowlist, got %d", delW.Code)
	}
}
