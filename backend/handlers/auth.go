package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/argon2"
)

type AuthHandler struct {
	DB *sql.DB
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Status string `json:"status"`
	Role   string `json:"role,omitempty"`
	UserID int    `json:"user_id,omitempty"`
	Name   string `json:"name,omitempty"`
}

func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON tidak valid", http.StatusBadRequest)
		return
	}

	var (
		userID       int
		role         string
		passwordHash string
		fullName     string
		status       string
	)

	err := h.DB.QueryRow(`
		SELECT
			u.id,
			r.name,
			u.password_hash,
			u.full_name,
			u.status
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.username = ?
	`, req.Username).Scan(
		&userID,
		&role,
		&passwordHash,
		&fullName,
		&status,
	)

	if err != nil || status != "ACTIVE" {
		writeJSON(w, http.StatusUnauthorized, LoginResponse{
			Status: "INVALID_CREDENTIALS",
		})
		return
	}

	if !VerifyPassword(req.Password, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, LoginResponse{
			Status: "INVALID_CREDENTIALS",
		})
		return
	}

	token, err := GenerateToken()

	if err != nil {
		http.Error(w, "Gagal membuat session", http.StatusInternalServerError)
		return
	}

	expires := time.Now().Add(8 * time.Hour)

	_, err = h.DB.Exec(`
		CREATE TABLE IF NOT EXISTS auth_sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)

	if err != nil {
		http.Error(w, "Gagal membuat session", http.StatusInternalServerError)
		return
	}

	_, err = h.DB.Exec(`
		INSERT INTO auth_sessions
			(token, user_id, expires_at)
		VALUES (?, ?, ?)
	`, token, userID, expires)

	if err != nil {
		http.Error(w, "Gagal menyimpan session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "wifi_management_session",
		Value:    token,
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	writeJSON(w, http.StatusOK, LoginResponse{
		Status: "OK",
		Role:   role,
		UserID: userID,
		Name:   fullName,
	})
}

func GeneratePassword(password string) (string, error) {
	salt := make([]byte, 16)

	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		1,
		64*1024,
		4,
		32,
	)

	return "argon2id$" +
		base64.RawStdEncoding.EncodeToString(salt) +
		"$" +
		base64.RawStdEncoding.EncodeToString(hash), nil
}

func VerifyPassword(password, encoded string) bool {
	const prefix = "argon2id$"

	if len(encoded) <= len(prefix) ||
		encoded[:len(prefix)] != prefix {
		return false
	}

	parts := splitHash(encoded)

	if len(parts) != 2 {
		return false
	}

	salt, err1 := base64.RawStdEncoding.DecodeString(parts[0])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[1])

	if err1 != nil || err2 != nil {
		return false
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		1,
		64*1024,
		4,
		uint32(len(expected)),
	)

	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func splitHash(encoded string) []string {
	result := make([]string, 0, 2)

	start := len("argon2id$")

	for i := start; i <= len(encoded); i++ {
		if i == len(encoded) || encoded[i] == '$' {
			result = append(result, encoded[start:i])
			start = i + 1
		}
	}

	return result
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func GenerateToken() (string, error) {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("wifi_management_session")
	if err == nil && cookie.Value != "" {
		_, _ = h.DB.Exec(
			`DELETE FROM auth_sessions WHERE token = ?`,
			cookie.Value,
		)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "wifi_management_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "OK",
		"message": "Logout berhasil",
	})
}
