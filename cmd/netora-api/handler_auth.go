package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/model"
)

// Handler: Login
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	user, err := s.store.GetUserByUsername(r.Context(), req.Username)
	// Always run exactly one bcrypt compare so response time does not reveal whether the user exists.
	hash := dummyPasswordHash
	if err == nil && user != nil {
		hash = user.PasswordHash
	}
	passwordOK := auth.CheckPassword(hash, req.Password)
	if err != nil || user == nil || !passwordOK {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	token, err := auth.GenerateToken(user, s.jwtSecret, 24*time.Hour)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
		Actor:    user.Username,
		Action:   "USER_LOGIN",
		Target:   user.ID,
		ClientIP: r.RemoteAddr,
		Status:   "SUCCESS",
	})

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"full_name": user.FullName,
		},
	})
}

// Handler: Logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := "MANUAL"
	if req.Reason != "" {
		reason = req.Reason
	}

	tokenStr := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	} else if qToken := r.URL.Query().Get("token"); qToken != "" {
		tokenStr = qToken
	}

	if tokenStr != "" {
		if claims, err := auth.ValidateToken(tokenStr, s.jwtSecret); err == nil && claims != nil {
			_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
				Actor:    claims.Username,
				Action:   "USER_LOGOUT",
				Target:   claims.UserID,
				ClientIP: r.RemoteAddr,
				Status:   reason,
			})
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
		"reason":  reason,
	})
}

// Handler: Me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(userClaimsKey).(*auth.Claims)
	if !ok {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.writeJSON(w, http.StatusOK, claims)
}

type updateProfileRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Handler: Update Profile / Credentials (Username & Password)
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	claims, ok := r.Context().Value(userClaimsKey).(*auth.Claims)
	if !ok {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// 1. Fetch current user record
	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		user, err = s.store.GetUserByUsername(r.Context(), claims.Username)
		if err != nil {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "user record not found"})
			return
		}
	}

	// 2. Validate current password
	if strings.TrimSpace(req.CurrentPassword) == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password saat ini wajib diisi untuk verifikasi keamanan"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Password saat ini salah"})
		return
	}

	// 3. Validate new username if provided
	targetUsername := strings.TrimSpace(req.Username)
	if targetUsername == "" {
		targetUsername = user.Username
	}
	if len(targetUsername) < 3 {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Username minimal harus 3 karakter"})
		return
	}
	if targetUsername != user.Username {
		existing, _ := s.store.GetUserByUsername(r.Context(), targetUsername)
		if existing != nil && existing.ID != user.ID {
			s.writeJSON(w, http.StatusConflict, map[string]string{"error": "Username '" + targetUsername + "' sudah digunakan oleh akun lain"})
			return
		}
	}

	// 4. Validate new password if provided
	targetPasswordHash := user.PasswordHash
	passwordChanged := false
	if strings.TrimSpace(req.NewPassword) != "" {
		if len(req.NewPassword) < 6 {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password baru minimal harus 6 karakter"})
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mengenkripsi password baru"})
			return
		}
		targetPasswordHash = hash
		passwordChanged = true
	}

	// 5. Update user credentials in database
	if err := s.store.UpdateUserCredentials(r.Context(), user.ID, targetUsername, targetPasswordHash); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal memperbarui akun: " + err.Error()})
		return
	}

	// 6. Record Audit Log
	details := fmt.Sprintf("username updated to '%s'", targetUsername)
	if passwordChanged {
		details += ", password changed"
	}
	_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
		Actor:    user.Username,
		Action:   "USER_CREDENTIALS_UPDATED",
		Target:   user.ID,
		Details:  details,
		ClientIP: r.RemoteAddr,
		Status:   "SUCCESS",
	})

	// 7. Issue refreshed token with new username
	user.Username = targetUsername
	user.PasswordHash = targetPasswordHash
	newToken, _ := auth.GenerateToken(user, s.jwtSecret, 24*time.Hour)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pengaturan akun berhasil disimpan",
		"token":   newToken,
		"user": map[string]string{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"full_name": user.FullName,
		},
	})
}
