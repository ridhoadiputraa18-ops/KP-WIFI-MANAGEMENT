package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type contextKey string

const userContextKey contextKey = "auth_user"

type User struct {
	ID       int
	Username string
	FullName string
	Role     string
}

func CurrentUser(r *http.Request) (*User, bool) {
	user, ok := r.Context().Value(userContextKey).(*User)
	return user, ok
}

type Auth struct {
	DB *sql.DB
}

func NewAuth(db *sql.DB) *Auth {
	return &Auth{DB: db}
}

func (a *Auth) GetUser(r *http.Request) (*User, bool) {
	cookie, err := r.Cookie("wifi_management_session")
	if err != nil || cookie.Value == "" {
		return nil, false
	}

	var user User
	var expiresAt time.Time

	err = a.DB.QueryRow(`
		SELECT
			u.id,
			u.username,
			u.full_name,
			ro.name,
			s.expires_at
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		JOIN roles ro ON ro.id = u.role_id
		WHERE s.token = $1
		  AND u.status = 'ACTIVE'
	`, cookie.Value).Scan(
		&user.ID,
		&user.Username,
		&user.FullName,
		&user.Role,
		&expiresAt,
	)

	if err != nil {
		return nil, false
	}

	if time.Now().After(expiresAt) {
		_, _ = a.DB.Exec(
			`DELETE FROM auth_sessions WHERE token = $1`,
			cookie.Value,
		)
		return nil, false
	}

	return &user, true
}

func (a *Auth) RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.GetUser(r)

		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"status":  "UNAUTHORIZED",
				"message": "Silakan login terlebih dahulu",
			})
			return
		}

		ctx := context.WithValue(
			r.Context(),
			userContextKey,
			user,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.GetUser(r)

		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"status":  "UNAUTHORIZED",
				"message": "Silakan login terlebih dahulu",
			})
			return
		}

		if user.Role != role {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"status":  "FORBIDDEN",
				"message": "Anda tidak memiliki akses",
			})
			return
		}

		ctx := context.WithValue(
			r.Context(),
			userContextKey,
			user,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
