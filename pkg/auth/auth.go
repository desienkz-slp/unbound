package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Standard RBAC Roles
const (
	RoleSuperAdmin = "SUPER_ADMIN"
	RoleAdmin      = "ADMIN"
	RoleOperator   = "OPERATOR"
	RoleNOC        = "NOC"
	RoleAuditor    = "AUDITOR"
	RoleReadOnly   = "READONLY"
)

// User represents an authenticated operator in the system.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Email        string    `json:"email"`
	FullName     string    `json:"full_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Claims defines custom JWT authentication claims.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// RolePermissions maps each role to allowed granular permissions.
var RolePermissions = map[string][]string{
	RoleSuperAdmin: {"*"},
	RoleAdmin: {
		"nodes:read", "nodes:write", "config:read", "config:write", "config:apply",
		"config:rollback", "policy:read", "policy:write", "audit:read", "users:read", "users:write",
	},
	RoleOperator: {
		"nodes:read", "config:read", "config:apply", "config:rollback",
		"policy:read", "policy:write", "audit:read",
	},
	RoleNOC: {
		"nodes:read", "config:read", "audit:read", "diagnostics:execute", "stats:read",
	},
	RoleAuditor: {
		"audit:read", "config:read", "nodes:read",
	},
	RoleReadOnly: {
		"nodes:read", "config:read", "policy:read",
	},
}

// HashPassword generates a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken generates a signed JWT token.
func GenerateToken(user *User, secret string, duration time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret key cannot be empty")
	}
	if duration <= 0 {
		duration = 24 * time.Hour
	}

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "netora-dns",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signed, nil
}

// ValidateToken verifies and parses a JWT token string.
func ValidateToken(tokenStr, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret key cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// HasPermission checks if the given role has the required permission.
func HasPermission(role, requiredPermission string) bool {
	perms, exists := RolePermissions[role]
	if !exists {
		return false
	}
	for _, p := range perms {
		if p == "*" || p == requiredPermission {
			return true
		}
	}
	return false
}
