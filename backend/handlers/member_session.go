package handlers

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

type MemberSessionHandler struct {
	DB *sql.DB
}

func NewMemberSessionHandler(db *sql.DB) *MemberSessionHandler {
	return &MemberSessionHandler{DB: db}
}

type CreateSessionRequest struct {
	IP         string `json:"client_ip"`
	MAC        string `json:"client_mac"`
	DeviceName string `json:"device_name"`
}

func (h *MemberSessionHandler) Start(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	var req CreateSessionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_JSON",
		})
		return
	}

	req.IP = strings.TrimSpace(req.IP)
	req.MAC = strings.TrimSpace(req.MAC)
	req.DeviceName = strings.TrimSpace(req.DeviceName)

	if req.IP != "" && net.ParseIP(req.IP) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "INVALID_IP",
			"message": "Format IP address tidak valid",
		})
		return
	}

	if req.MAC != "" {
		if _, err := net.ParseMAC(req.MAC); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_MAC",
				"message": "Format MAC address tidak valid",
			})
			return
		}
	}

	var memberID int
	var memberStatus string

	err := h.DB.QueryRow(`
		SELECT id, status
		FROM members
		WHERE user_id = $1
	`, userID).Scan(&memberID, &memberStatus)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status": "MEMBER_NOT_FOUND",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	if memberStatus != "ACTIVE" {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"status":  "MEMBER_INACTIVE",
			"message": "Akun Member tidak aktif",
		})
		return
	}

	var accessStart sql.NullString
	var accessEnd sql.NullString

	err = h.DB.QueryRow(`
		SELECT access_start, access_end
		FROM members
		WHERE id = $1
	`, memberID).Scan(&accessStart, &accessEnd)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	now := time.Now().UTC()

	if accessStart.Valid && accessStart.String != "" {
		start, err := time.Parse(time.RFC3339, accessStart.String)
		if err == nil && now.Before(start) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"status":  "ACCESS_NOT_STARTED",
				"message": "Masa akses Member belum dimulai",
			})
			return
		}
	}

	if accessEnd.Valid && accessEnd.String != "" {
		end, err := time.Parse(time.RFC3339, accessEnd.String)
		if err == nil && !now.Before(end) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"status":  "ACCESS_EXPIRED",
				"message": "Masa akses Member sudah berakhir",
			})
			return
		}
	}

	var activeSessionID int

	err = h.DB.QueryRow(`
		SELECT id
		FROM sessions
		WHERE user_id = $1
		  AND status = 'ONLINE'
		ORDER BY started_at DESC
		LIMIT 1
	`, userID).Scan(&activeSessionID)

	if err == nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"status":     "SESSION_ALREADY_ACTIVE",
			"message":    "Member masih memiliki session aktif",
			"session_id": activeSessionID,
		})
		return
	}

	if err != sql.ErrNoRows {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	var sessionID int64

	err = h.DB.QueryRow(`
                INSERT INTO sessions (
                        user_id,
                        client_ip,
                        client_mac,
                        started_at,
                        status
                )
                VALUES ($1, $2, $3, $4, 'ONLINE')
                RETURNING id
        `,
		userID,
		nullString(req.IP),
		nullString(req.MAC),
		now.Format(time.RFC3339),
	).Scan(&sessionID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":     "OK",
		"message":    "Session Member berhasil dimulai",
		"session_id": sessionID,
		"session": map[string]interface{}{
			"user_id":     userID,
			"member_id":   memberID,
			"start_time":  now.Format(time.RFC3339),
			"status":      "ONLINE",
			"ip_address":  nullableValue(req.IP),
			"mac_address": nullableValue(req.MAC),
			"device_name": nullableValue(req.DeviceName),
		},
	})
}

func nullableValue(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	return value
}
