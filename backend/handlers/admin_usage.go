package handlers

import (
	"database/sql"
	"net/http"
)

type AdminUsageHandler struct {
	DB *sql.DB
}

func NewAdminUsageHandler(db *sql.DB) *AdminUsageHandler {
	return &AdminUsageHandler{DB: db}
}

func (h *AdminUsageHandler) History(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			ul.id,
			ul.session_id,
			COALESCE(u.username, ''),
			COALESCE(u.full_name, ''),
			ul.upload_bytes,
			ul.download_bytes,
			ul.total_bytes,
			ul.recorded_at
		FROM usage_logs ul
		INNER JOIN sessions s ON s.id = ul.session_id
		LEFT JOIN users u ON u.id = s.user_id
		ORDER BY ul.recorded_at DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}
	defer rows.Close()

	type Usage struct {
		ID            int64  `json:"id"`
		SessionID     int64  `json:"session_id"`
		Username      string `json:"username"`
		FullName      string `json:"full_name"`
		UploadBytes   int64  `json:"upload_bytes"`
		DownloadBytes int64  `json:"download_bytes"`
		TotalBytes    int64  `json:"total_bytes"`
		RecordedAt    string `json:"recorded_at"`
	}

	usages := []Usage{}

	for rows.Next() {
		var u Usage

		err := rows.Scan(
			&u.ID,
			&u.SessionID,
			&u.Username,
			&u.FullName,
			&u.UploadBytes,
			&u.DownloadBytes,
			&u.TotalBytes,
			&u.RecordedAt,
		)

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"status": "DATABASE_ERROR",
			})
			return
		}

		usages = append(usages, u)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"total":  len(usages),
		"usage":  usages,
	})
}
