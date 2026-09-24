package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"kp-wifi-management/middleware"
)

type GuestAdminHandler struct {
	DB *sql.DB
}

func NewGuestAdminHandler(db *sql.DB) *GuestAdminHandler {
	return &GuestAdminHandler{DB: db}
}

type guestConfigRequest struct {
	SSID                 string `json:"ssid"`
	Password             string `json:"password"`
	GuestDurationMinutes int    `json:"guest_duration_minutes"`
	Status               string `json:"status"`
}

type guestRequest struct {
	Name string `json:"name"`
}

func (h *GuestAdminHandler) Config(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok || user.Role != "ADMIN" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"status":  "ERROR",
			"message": "Unauthorized",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getConfig(w)
	case http.MethodPost:
		h.saveConfig(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"status":  "ERROR",
			"message": "Method not allowed",
		})
	}
}

func (h *GuestAdminHandler) getConfig(w http.ResponseWriter) {
	row := h.DB.QueryRow(`
		SELECT id, ssid, network_type, password_encrypted,
		       guest_duration_minutes, status, created_at, updated_at
		FROM wifi_configs
		WHERE network_type = 'GUEST'
		ORDER BY id DESC
		LIMIT 1
	`)

	var (
		id       int
		ssid     string
		network  string
		encPass  sql.NullString
		duration int
		status   string
		created  string
		updated  string
	)

	err := row.Scan(
		&id,
		&ssid,
		&network,
		&encPass,
		&duration,
		&status,
		&created,
		&updated,
	)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "OK",
			"config": nil,
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	password := ""
	if encPass.Valid && encPass.String != "" {
		if value, err := decryptPassword(encPass.String); err == nil {
			password = value
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "OK",
		"config": map[string]any{
			"id":                     id,
			"ssid":                   ssid,
			"network_type":           network,
			"password":               password,
			"guest_duration_minutes": duration,
			"status":                 status,
			"created_at":             created,
			"updated_at":             updated,
		},
	})
}

func (h *GuestAdminHandler) saveConfig(w http.ResponseWriter, r *http.Request) {
	var req guestConfigRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "ERROR",
			"message": "JSON tidak valid",
		})
		return
	}

	req.SSID = strings.TrimSpace(req.SSID)
	req.Password = strings.TrimSpace(req.Password)
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))

	if req.SSID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "ERROR",
			"message": "SSID wajib diisi",
		})
		return
	}

	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "ERROR",
			"message": "Password Wi-Fi minimal 8 karakter",
		})
		return
	}

	if req.GuestDurationMinutes <= 0 {
		req.GuestDurationMinutes = 120
	}

	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	encrypted, err := encryptPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": "Gagal mengenkripsi password",
		})
		return
	}

	var existingID int

	err = h.DB.QueryRow(`
		SELECT id
		FROM wifi_configs
		WHERE network_type = 'GUEST'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&existingID)

	if err == sql.ErrNoRows {
		var id int64

		err = h.DB.QueryRow(`
                        INSERT INTO wifi_configs
                        (ssid, network_type, password_encrypted,
                         guest_duration_minutes, status)
                        VALUES ($1, 'GUEST', $2, $3, $4)
                        RETURNING id
                `,
			req.SSID,
			encrypted,
			req.GuestDurationMinutes,
			req.Status,
		).Scan(&id)

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "ERROR",
				"message": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"status":  "OK",
			"message": "Konfigurasi Guest Wi-Fi berhasil dibuat",
			"id":      id,
		})
		return

	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	_, err = h.DB.Exec(`
		UPDATE wifi_configs
            SET ssid = $1,
                password_encrypted = $2,
                guest_duration_minutes = $3,
                status = $4,
                updated_at = CURRENT_TIMESTAMP
            WHERE id = $5
	`,
		req.SSID,
		encrypted,
		req.GuestDurationMinutes,
		req.Status,
		existingID,
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "OK",
		"message": "Konfigurasi Guest Wi-Fi berhasil diperbarui",
		"id":      existingID,
	})
}

func (h *GuestAdminHandler) Guests(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok || user.Role != "ADMIN" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"status":  "ERROR",
			"message": "Unauthorized",
		})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"status":  "ERROR",
			"message": "Method not allowed",
		})
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			g.id,
			g.name,
			g.access_start,
			g.access_end,
			g.status,
			g.created_at,
			COUNT(s.id) AS session_count
		FROM guests g
		LEFT JOIN sessions s ON s.guest_id = g.id
		GROUP BY
			g.id,
			g.name,
			g.access_start,
			g.access_end,
			g.status,
			g.created_at
		ORDER BY g.id DESC
	`)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	defer rows.Close()

	guests := make([]map[string]any, 0)

	for rows.Next() {
		var (
			id           int
			name         string
			accessStart  sql.NullString
			accessEnd    sql.NullString
			status       string
			createdAt    string
			sessionCount int
		)

		if err := rows.Scan(
			&id,
			&name,
			&accessStart,
			&accessEnd,
			&status,
			&createdAt,
			&sessionCount,
		); err != nil {
			continue
		}

		guests = append(guests, map[string]any{
			"id":            id,
			"name":          name,
			"access_start":  accessStart.String,
			"access_end":    accessEnd.String,
			"status":        status,
			"created_at":    createdAt,
			"session_count": sessionCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "OK",
		"total":  len(guests),
		"guests": guests,
	})
}

func (h *GuestAdminHandler) CreateGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"status":  "ERROR",
			"message": "Method not allowed",
		})
		return
	}

	var req guestRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "ERROR",
			"message": "JSON tidak valid",
		})
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "ERROR",
			"message": "Nama guest wajib diisi",
		})
		return
	}

	var duration int

	err := h.DB.QueryRow(`
		SELECT guest_duration_minutes
		FROM wifi_configs
		WHERE network_type = 'GUEST'
		  AND status = 'ACTIVE'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&duration)

	if err == sql.ErrNoRows {
		duration = 120
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	start := time.Now().UTC()
	end := start.Add(time.Duration(duration) * time.Minute)

	var id int64

	err = h.DB.QueryRow(`
                INSERT INTO guests
                (name, access_start, access_end, status)
                VALUES ($1, $2, $3, 'ACTIVE')
                RETURNING id
        `,
		req.Name,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	).Scan(&id)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":       "OK",
		"id":           id,
		"name":         req.Name,
		"access_start": start.Format(time.RFC3339),
		"access_end":   end.Format(time.RFC3339),
		"duration_min": duration,
	})
}

func (h *GuestAdminHandler) GuestSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok || user.Role != "ADMIN" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"status":  "ERROR",
			"message": "Unauthorized",
		})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"status":  "ERROR",
			"message": "Method not allowed",
		})
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			s.id,
			s.guest_id,
			g.name,
			s.client_ip,
			s.client_mac,
			s.started_at,
			s.ended_at,
			s.status
		FROM sessions s
		INNER JOIN guests g ON g.id = s.guest_id
		ORDER BY s.id DESC
	`)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	defer rows.Close()

	sessions := make([]map[string]any, 0)

	for rows.Next() {
		var (
			id      int
			guestID int
			name    string
			ip      sql.NullString
			mac     sql.NullString
			started string
			ended   sql.NullString
			status  string
		)

		if err := rows.Scan(
			&id,
			&guestID,
			&name,
			&ip,
			&mac,
			&started,
			&ended,
			&status,
		); err != nil {
			continue
		}

		sessions = append(sessions, map[string]any{
			"id":         id,
			"guest_id":   guestID,
			"name":       name,
			"client_ip":  ip.String,
			"client_mac": mac.String,
			"started_at": started,
			"ended_at":   ended.String,
			"status":     status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "OK",
		"total":    len(sessions),
		"sessions": sessions,
	})
}

func encryptionKey() ([]byte, error) {
	path := "wifi_config.key"

	data, err := os.ReadFile(path)
	if err == nil && len(data) == 32 {
		return data, nil
	}

	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}

	return key, nil
}

func encryptPassword(password string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	data := gcm.Seal(nonce, nonce, []byte(password), nil)

	return base64.RawStdEncoding.EncodeToString(data), nil
}

func decryptPassword(encoded string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}

	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()

	if len(raw) < nonceSize {
		return "", fmt.Errorf("encrypted password invalid")
	}

	nonce := raw[:nonceSize]
	data := raw[nonceSize:]

	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

func guestIDFromPath(path string) (int, error) {
	value := strings.Trim(path, "/")
	parts := strings.Split(value, "/")

	if len(parts) == 0 {
		return 0, fmt.Errorf("guest id tidak ditemukan")
	}

	return strconv.Atoi(parts[len(parts)-1])
}
