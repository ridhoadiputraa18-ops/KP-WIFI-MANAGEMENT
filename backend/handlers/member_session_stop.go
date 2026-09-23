package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MemberSessionStopHandler struct {
	DB *sql.DB
}

func NewMemberSessionStopHandler(db *sql.DB) *MemberSessionStopHandler {
	return &MemberSessionStopHandler{DB: db}
}

func (h *MemberSessionStopHandler) Stop(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	sessionIDStr := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionIDStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "SESSION_ID_REQUIRED",
			"message": "session_id wajib diisi",
		})
		return
	}

	sessionID, err := strconv.Atoi(sessionIDStr)
	if err != nil || sessionID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "INVALID_SESSION_ID",
			"message": "session_id tidak valid",
		})
		return
	}

	var status string

	err = h.DB.QueryRow(`
		SELECT status
		FROM sessions
		WHERE id = ?
		  AND user_id = ?
	`, sessionID, userID).Scan(&status)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status":  "SESSION_NOT_FOUND",
			"message": "Session tidak ditemukan",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}

	if status != "ONLINE" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "SESSION_NOT_ACTIVE",
			"message": "Session sudah tidak aktif",
		})
		return
	}

	endedAt := time.Now().UTC()

	result, err := h.DB.Exec(`
		UPDATE sessions
		SET ended_at = ?,
		    status = 'OFFLINE'
		WHERE id = ?
		  AND user_id = ?
		  AND status = 'ONLINE'
	`, endedAt.Format(time.RFC3339), sessionID, userID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}

	if affected == 0 {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "SESSION_NOT_ACTIVE",
			"message": "Session sudah tidak aktif",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "OK",
		"message":    "Session berhasil dihentikan",
		"session_id": sessionID,
		"ended_at":   endedAt.Format(time.RFC3339),
	})
}
