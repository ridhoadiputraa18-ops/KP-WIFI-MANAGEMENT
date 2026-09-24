package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type MemberActiveSessionHandler struct {
	DB *sql.DB
}

func NewMemberActiveSessionHandler(db *sql.DB) *MemberActiveSessionHandler {
	return &MemberActiveSessionHandler{DB: db}
}

func (h *MemberActiveSessionHandler) Get(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	rows, err := h.DB.Query(`
                SELECT
                        id,
                        started_at,
                        ended_at,
                        status,
                        COALESCE(client_ip, ''),
                        COALESCE(client_mac, '')
                FROM sessions
                WHERE user_id = $1
                  AND status = 'ONLINE'
                ORDER BY started_at DESC
        `, userID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}
	defer rows.Close()

	type Session struct {
		ID         int    `json:"id"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
		Status     string `json:"status"`
		IPAddress  string `json:"ip_address"`
		MACAddress string `json:"mac_address"`
		DeviceName string `json:"device_name"`
		Duration   string `json:"duration"`
	}

	sessions := []Session{}

	for rows.Next() {
		var (
			session   Session
			startTime string
			endTime   sql.NullString
		)

		err := rows.Scan(
			&session.ID,
			&startTime,
			&endTime,
			&session.Status,
			&session.IPAddress,
			&session.MACAddress,
		)

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"status":  "DATABASE_ERROR",
				"message": err.Error(),
			})
			return
		}

		session.StartTime = startTime
		session.DeviceName = ""

		if endTime.Valid {
			session.EndTime = endTime.String
		}

		start := parseDatabaseTime(startTime)
		if !start.IsZero() {
			duration := time.Since(start)
			if duration < 0 {
				duration = 0
			}
			session.Duration = formatDuration(duration)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "OK",
		"total":    len(sessions),
		"sessions": sessions,
	})
}

func parseDatabaseTime(value string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t
		}
	}

	return time.Time{}
}

func formatDuration(d time.Duration) string {
	totalSeconds := int64(d.Seconds())

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d jam %02d menit %02d detik", hours, minutes, seconds)
}
