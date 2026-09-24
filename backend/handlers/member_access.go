package handlers

import (
	"database/sql"
	"net/http"
	"time"
)

type MemberAccessHandler struct {
	DB *sql.DB
}

func NewMemberAccessHandler(db *sql.DB) *MemberAccessHandler {
	return &MemberAccessHandler{DB: db}
}

func (h *MemberAccessHandler) Get(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	var (
		memberID     int
		memberCode   string
		memberStatus string
		accessStart  sql.NullString
		accessEnd    sql.NullString
	)

	err := h.DB.QueryRow(`
		SELECT
			m.id,
			m.member_code,
			m.status,
			m.access_start,
			m.access_end
		FROM members m
		WHERE m.user_id = $1
	`, userID).Scan(
		&memberID,
		&memberCode,
		&memberStatus,
		&accessStart,
		&accessEnd,
	)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status": "MEMBER_ACCESS_NOT_FOUND",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	result := "NO_ACCESS_WINDOW"

	var startTime *time.Time
	var endTime *time.Time

	if accessStart.Valid && accessStart.String != "" {
		if parsed, err := time.Parse(time.RFC3339, accessStart.String); err == nil {
			startTime = &parsed
		}
	}

	if accessEnd.Valid && accessEnd.String != "" {
		if parsed, err := time.Parse(time.RFC3339, accessEnd.String); err == nil {
			endTime = &parsed
		}
	}

	now := time.Now()

	if memberStatus != "ACTIVE" {
		result = "MEMBER_INACTIVE"
	} else if startTime != nil && endTime != nil {
		switch {
		case now.Before(*startTime):
			result = "NOT_STARTED"
		case now.After(*endTime):
			result = "EXPIRED"
		default:
			result = "ACTIVE"
		}
	} else if startTime != nil && endTime == nil {
		if now.Before(*startTime) {
			result = "NOT_STARTED"
		} else {
			result = "ACTIVE"
		}
	} else if startTime == nil && endTime != nil {
		if now.After(*endTime) {
			result = "EXPIRED"
		} else {
			result = "ACTIVE"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"access": map[string]interface{}{
			"member_id":     memberID,
			"member_code":   memberCode,
			"member_status": memberStatus,
			"access_start":  nullableString(accessStart),
			"access_end":    nullableString(accessEnd),
			"access_state":  result,
			"checked_at":    now.Format(time.RFC3339),
		},
	})
}

func nullableString(value sql.NullString) interface{} {
	if !value.Valid || value.String == "" {
		return nil
	}

	return value.String
}
