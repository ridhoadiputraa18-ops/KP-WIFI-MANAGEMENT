package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"kp-wifi-management/middleware"
	"kp-wifi-management/services"
)

type TechnicianAdminHandler struct {
	DB *sql.DB
}

func NewTechnicianAdminHandler(db *sql.DB) *TechnicianAdminHandler {
	return &TechnicianAdminHandler{DB: db}
}

type CreateTechnicianRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

func (h *TechnicianAdminHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "METHOD_NOT_ALLOWED",
			"message": "Method tidak diizinkan",
		})
		return
	}

	rows, err := h.DB.Query(`
		SELECT u.id, u.username, u.full_name,
		       COALESCE(u.email, ''), u.status, u.created_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE r.name = 'TECHNICIAN'
		ORDER BY u.id DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal mengambil data teknisi",
		})
		return
	}
	defer rows.Close()

	technicians := []map[string]interface{}{}

	for rows.Next() {
		var (
			id        int
			username  string
			fullName  string
			email     string
			status    string
			createdAt string
		)

		if err := rows.Scan(
			&id,
			&username,
			&fullName,
			&email,
			&status,
			&createdAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"status":  "ERROR",
				"message": "Gagal membaca data teknisi",
			})
			return
		}

		technicians = append(technicians, map[string]interface{}{
			"id":         id,
			"username":   username,
			"full_name":  fullName,
			"email":      email,
			"status":     status,
			"created_at": createdAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "OK",
		"total":       len(technicians),
		"technicians": technicians,
	})
}

func (h *TechnicianAdminHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	var req CreateTechnicianRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "INVALID_JSON",
			"message": "JSON tidak valid",
		})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || req.Password == "" || req.FullName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "VALIDATION_ERROR",
			"message": "Username, password, dan nama lengkap wajib diisi",
		})
		return
	}

	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "VALIDATION_ERROR",
			"message": "Password minimal 8 karakter",
		})
		return
	}

	var roleID int

	err := h.DB.QueryRow(`
		SELECT id FROM roles WHERE name = 'TECHNICIAN'
	`).Scan(&roleID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Role TECHNICIAN tidak ditemukan",
		})
		return
	}

	var existing int

	err = h.DB.QueryRow(`
		SELECT COUNT(*) FROM users WHERE username = ?
	`, req.Username).Scan(&existing)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status": "ERROR",
		})
		return
	}

	if existing > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"status":  "USERNAME_EXISTS",
			"message": "Username sudah digunakan",
		})
		return
	}

	passwordHash, err := GeneratePassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal membuat password hash",
		})
		return
	}

	var result sql.Result

	if req.Email == "" {
		result, err = h.DB.Exec(`
			INSERT INTO users
				(role_id, username, password_hash, full_name, status)
			VALUES (?, ?, ?, ?, 'ACTIVE')
		`, roleID, req.Username, passwordHash, req.FullName)
	} else {
		result, err = h.DB.Exec(`
			INSERT INTO users
				(role_id, username, password_hash, full_name, email, status)
			VALUES (?, ?, ?, ?, ?, 'ACTIVE')
		`, roleID, req.Username, passwordHash, req.FullName, req.Email)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal membuat akun teknisi",
		})
		return
	}

	id, _ := result.LastInsertId()

	// Audit log pembuatan akun teknisi.
	if currentUser, ok := middleware.CurrentUser(r); ok {
		logger := services.NewAuditLogger(h.DB)
		userID := int64(currentUser.ID)

		if err := logger.Log(
			r,
			&userID,
			"TECHNICIAN_CREATE",
			"USER",
			&id,
			"Admin membuat akun teknisi: "+req.Username,
		); err != nil {
			// Audit gagal tidak membatalkan akun yang sudah berhasil dibuat.
			// Kegagalan audit dicatat di log server.
			println("audit log error:", err.Error())
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "OK",
		"message":  "Akun teknisi berhasil dibuat",
		"id":       id,
		"role":     "TECHNICIAN",
		"username": req.Username,
	})
}

func (h *TechnicianAdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/admin/technicians/")
	id, err := strconv.Atoi(idText)

	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "INVALID_ID",
			"message": "ID teknisi tidak valid",
		})
		return
	}

	var technicianUsername string

	err = h.DB.QueryRow(`
		SELECT username
		FROM users
		WHERE id = ?
		AND role_id = (SELECT id FROM roles WHERE name = 'TECHNICIAN')
	`, id).Scan(&technicianUsername)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "NOT_FOUND",
			"message": "Teknisi tidak ditemukan",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal mengambil data teknisi",
		})
		return
	}

	result, err := h.DB.Exec(`
		DELETE FROM users
		WHERE id = ?
		AND role_id = (SELECT id FROM roles WHERE name = 'TECHNICIAN')
	`, id)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status": "ERROR",
		})
		return
	}

	affected, _ := result.RowsAffected()

	if affected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "NOT_FOUND",
			"message": "Teknisi tidak ditemukan",
		})
		return
	}

	// Audit log penghapusan akun teknisi.
	if currentUser, ok := middleware.CurrentUser(r); ok {
		logger := services.NewAuditLogger(h.DB)
		userID := int64(currentUser.ID)
		targetID := int64(id)

		if err := logger.Log(
			r,
			&userID,
			"TECHNICIAN_DELETE",
			"USER",
			&targetID,
			"Admin menghapus akun teknisi: "+technicianUsername,
		); err != nil {
			// Kegagalan audit tidak membatalkan penghapusan yang sudah berhasil.
			println("audit log error:", err.Error())
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"message": "Akun teknisi berhasil dihapus",
	})
}
