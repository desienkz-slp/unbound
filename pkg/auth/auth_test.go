package auth

import (
	"testing"
	"time"
)

func TestAuth_PasswordHashing(t *testing.T) {
	password := "SecretP@ssw0rd123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPassword(hash, password) {
		t.Errorf("expected correct password to match hash")
	}

	if CheckPassword(hash, "WrongPassword") {
		t.Errorf("expected wrong password to fail check")
	}
}

func TestAuth_JWTGenerationAndValidation(t *testing.T) {
	secret := "test-secret-key-12345678901234567890"
	user := &User{
		ID:       "usr-1",
		Username: "admin",
		Role:     RoleAdmin,
	}

	token, err := GenerateToken(user, secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != user.ID || claims.Username != user.Username || claims.Role != user.Role {
		t.Errorf("claims mismatch: %+v", claims)
	}

	// Validate with wrong secret fails
	_, err = ValidateToken(token, "wrong-secret-key")
	if err == nil {
		t.Errorf("expected validation to fail with wrong secret")
	}
}

func TestAuth_RBACPermissions(t *testing.T) {
	tests := []struct {
		role       string
		permission string
		expected   bool
	}{
		{RoleSuperAdmin, "nodes:write", true},
		{RoleSuperAdmin, "anything:random", true},
		{RoleAdmin, "nodes:write", true},
		{RoleAdmin, "users:write", true},
		{RoleOperator, "config:apply", true},
		{RoleOperator, "users:write", false},
		{RoleNOC, "diagnostics:execute", true},
		{RoleNOC, "config:apply", false},
		{RoleAuditor, "audit:read", true},
		{RoleAuditor, "config:write", false},
		{RoleReadOnly, "nodes:read", true},
		{RoleReadOnly, "nodes:write", false},
	}

	for _, tt := range tests {
		got := HasPermission(tt.role, tt.permission)
		if got != tt.expected {
			t.Errorf("HasPermission(%s, %s) = %v; expected %v", tt.role, tt.permission, got, tt.expected)
		}
	}
}
