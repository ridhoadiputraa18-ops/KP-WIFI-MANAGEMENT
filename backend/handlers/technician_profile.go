package handlers

import (
	"database/sql"
	"net/http"

	"kp-wifi-management/middleware"
)

type TechnicianProfileHandler struct {
	DB *sql.DB
}

func NewTechnicianProfileHandler(db *sql.DB) *TechnicianProfileHandler {
	return &TechnicianProfileHandler{DB: db}
}

func (h *TechnicianProfileHandler) Profile(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)

	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"status":  "UNAUTHORIZED",
			"message": "Silakan login terlebih dahulu",
		})
		return
	}

	var username, fullName, email, status string

	err := h.DB.QueryRow(`
		SELECT username, full_name, COALESCE(email, ''), status
		FROM users
		WHERE id = ?
		  AND role_id = (
			SELECT id FROM roles WHERE name = 'TECHNICIAN'
		  )
	`, user.ID).Scan(&username, &fullName, &email, &status)

	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"status":  "NOT_FOUND",
				"message": "Profil teknisi tidak ditemukan",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal mengambil profil teknisi",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"user": map[string]interface{}{
			"id":        user.ID,
			"username":  username,
			"full_name": fullName,
			"email":     email,
			"status":    status,
			"role":      "TECHNICIAN",
		},
	})
}
