package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type MemberUsageHandler struct {
	DB *sql.DB
}

func NewMemberUsageHandler(db *sql.DB) *MemberUsageHandler {
	return &MemberUsageHandler{DB: db}
}

type UsageRequest struct {
	SessionID     int64 `json:"session_id"`
	UploadBytes   int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
}

func (h *MemberUsageHandler) Record(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	var req UsageRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_JSON",
		})
		return
	}

	if req.SessionID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "INVALID_SESSION_ID",
			"message": "session_id wajib valid",
		})
		return
	}

	if req.UploadBytes < 0 || req.DownloadBytes < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "INVALID_USAGE",
			"message": "Nilai traffic tidak boleh negatif",
		})
		return
	}

	var sessionStatus string

	err := h.DB.QueryRow(`
		SELECT status
		FROM sessions
		WHERE id = ?
		  AND user_id = ?
	`, req.SessionID, userID).Scan(&sessionStatus)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status":  "SESSION_NOT_FOUND",
			"message": "Session tidak ditemukan",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	if strings.ToUpper(sessionStatus) != "ONLINE" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "SESSION_NOT_ACTIVE",
			"message": "Usage hanya dapat dicatat untuk session yang aktif",
		})
		return
	}

	totalBytes := req.UploadBytes + req.DownloadBytes

	result, err := h.DB.Exec(`
		INSERT INTO usage_logs (
			session_id,
			upload_bytes,
			download_bytes,
			total_bytes
		)
		VALUES (?, ?, ?, ?)
	`,
		req.SessionID,
		req.UploadBytes,
		req.DownloadBytes,
		totalBytes,
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":         "OK",
		"message":        "Usage berhasil dicatat",
		"usage_id":       id,
		"session_id":     req.SessionID,
		"upload_bytes":   req.UploadBytes,
		"download_bytes": req.DownloadBytes,
		"total_bytes":    totalBytes,
	})
}

func (h *MemberUsageHandler) History(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	sessionIDStr := strings.TrimSpace(r.URL.Query().Get("session_id"))

	query := `
		SELECT
			ul.id,
			ul.session_id,
			ul.upload_bytes,
			ul.download_bytes,
			ul.total_bytes,
			ul.recorded_at
		FROM usage_logs ul
		INNER JOIN sessions s ON s.id = ul.session_id
		WHERE s.user_id = ?
	`

	args := []interface{}{userID}

	if sessionIDStr != "" {
		sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
		if err != nil || sessionID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_SESSION_ID",
				"message": "session_id tidak valid",
			})
			return
		}

		query += " AND ul.session_id = ?"
		args = append(args, sessionID)
	}

	query += " ORDER BY ul.recorded_at DESC"

	rows, err := h.DB.Query(query, args...)
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
		UploadBytes   int64  `json:"upload_bytes"`
		DownloadBytes int64  `json:"download_bytes"`
		TotalBytes    int64  `json:"total_bytes"`
		RecordedAt    string `json:"recorded_at"`
	}

	usages := []Usage{}

	for rows.Next() {
		var usage Usage

		if err := rows.Scan(
			&usage.ID,
			&usage.SessionID,
			&usage.UploadBytes,
			&usage.DownloadBytes,
			&usage.TotalBytes,
			&usage.RecordedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"status": "DATABASE_ERROR",
			})
			return
		}

		usages = append(usages, usage)
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
