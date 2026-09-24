package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

type AdminAuditHandler struct {
	DB *sql.DB
}

func NewAdminAuditHandler(db *sql.DB) *AdminAuditHandler {
	return &AdminAuditHandler{DB: db}
}

func (h *AdminAuditHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "METHOD_NOT_ALLOWED",
			"message": "Method tidak diizinkan",
		})
		return
	}

	limit := 50
	offset := 0

	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if value := strings.TrimSpace(r.URL.Query().Get("offset")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	rows, err := h.DB.Query(`
		SELECT
			a.id,
			COALESCE(u.username, ''),
			COALESCE(u.full_name, ''),
			a.action,
			a.target_type,
			COALESCE(a.target_id, 0),
			a.description,
			COALESCE(a.ip_address, ''),
			a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id
		ORDER BY a.id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal mengambil audit log",
		})
		return
	}
	defer rows.Close()

	logs := []map[string]interface{}{}

	for rows.Next() {
		var (
			id          int64
			username    string
			fullName    string
			action      string
			targetType  string
			targetID    int64
			description string
			ipAddress   string
			createdAt   string
		)

		if err := rows.Scan(
			&id,
			&username,
			&fullName,
			&action,
			&targetType,
			&targetID,
			&description,
			&ipAddress,
			&createdAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"status":  "ERROR",
				"message": "Gagal membaca audit log",
			})
			return
		}

		logs = append(logs, map[string]interface{}{
			"id":          id,
			"username":    username,
			"full_name":   fullName,
			"action":      action,
			"target_type": targetType,
			"target_id":   targetID,
			"description": description,
			"ip_address":  ipAddress,
			"created_at":  createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal membaca audit log",
		})
		return
	}

	var total int

	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": "Gagal menghitung audit log",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"logs":   logs,
	})
}
