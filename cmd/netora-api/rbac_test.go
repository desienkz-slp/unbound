package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"netora-dns/pkg/auth"
)

// bearerFor issues a token for the given role using the server's secret.
func bearerFor(t *testing.T, s *Server, role string) string {
	t.Helper()
	tok, err := auth.GenerateToken(&auth.User{ID: "u1", Username: "u1", Role: role}, s.jwtSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + tok
}

func TestRequirePermEnforcesRBAC(t *testing.T) {
	s := &Server{jwtSecret: "test-secret"}
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	h := s.requirePerm("nodes:read", "config:apply", ok)

	cases := []struct {
		name, role, method string
		want               int
	}{
		{"readonly can GET", auth.RoleReadOnly, http.MethodGet, http.StatusOK},
		{"readonly cannot POST", auth.RoleReadOnly, http.MethodPost, http.StatusForbidden},
		{"auditor cannot POST", auth.RoleAuditor, http.MethodPost, http.StatusForbidden},
		{"operator can POST", auth.RoleOperator, http.MethodPost, http.StatusOK},
		{"superadmin can POST", auth.RoleSuperAdmin, http.MethodPost, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "/x", nil)
			req.Header.Set("Authorization", bearerFor(t, s, c.role))
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != c.want {
				t.Fatalf("got %d, want %d", rec.Code, c.want)
			}
		})
	}

	// No token -> 401
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", rec.Code)
	}
}

func TestLimitBodyRejectsOversizedPayload(t *testing.T) {
	s := &Server{}
	var readErr error
	h := s.LimitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, maxRequestBodyBytes+1024)
		_, readErr = r.Body.Read(buf)
		for readErr == nil { // drain until the limit trips
			_, readErr = r.Body.Read(buf)
		}
	}))
	big := strings.NewReader(strings.Repeat("a", maxRequestBodyBytes+1024))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", big))
	if readErr == nil || !strings.Contains(readErr.Error(), "too large") {
		t.Fatalf("expected 'request body too large', got %v", readErr)
	}
}

func TestCORSNoWildcardByDefault(t *testing.T) {
	t.Setenv("NETORA_CORS_ORIGINS", "http://localhost:5173")
	s := &Server{}
	h := s.CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for origin, want := range map[string]string{
		"http://localhost:5173": "http://localhost:5173",
		"http://evil.example":   "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Fatalf("origin %q: got %q, want %q", origin, got, want)
		}
	}
}
