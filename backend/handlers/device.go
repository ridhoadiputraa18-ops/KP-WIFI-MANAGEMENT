package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type DeviceHandler struct {
	DB *sql.DB
}

func NewDeviceHandler(db *sql.DB) *DeviceHandler {
	return &DeviceHandler{DB: db}
}

type Device struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	DeviceType  string  `json:"device_type"`
	Vendor      string  `json:"vendor"`
	Model       string  `json:"model"`
	IPAddress   string  `json:"ip_address"`
	MACAddress  string  `json:"mac_address"`
	APIEndpoint string  `json:"api_endpoint"`
	Status      string  `json:"status"`
	LastSeen    *string `json:"last_seen,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func writeDeviceJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeDeviceJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "ERROR",
			"message": "Method tidak diizinkan",
		})
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, name, device_type,
		       COALESCE(vendor,''), COALESCE(model,''),
		       COALESCE(ip_address,''), COALESCE(mac_address,''),
		       COALESCE(api_endpoint,''), status,
		       last_seen, created_at
		FROM devices
		ORDER BY id DESC
	`)
	if err != nil {
		writeDeviceJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}
	defer rows.Close()

	devices := []Device{}

	for rows.Next() {
		var d Device
		var lastSeen sql.NullString

		err := rows.Scan(
			&d.ID,
			&d.Name,
			&d.DeviceType,
			&d.Vendor,
			&d.Model,
			&d.IPAddress,
			&d.MACAddress,
			&d.APIEndpoint,
			&d.Status,
			&lastSeen,
			&d.CreatedAt,
		)

		if err != nil {
			writeDeviceJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"status":  "ERROR",
				"message": err.Error(),
			})
			return
		}

		if lastSeen.Valid {
			value := lastSeen.String
			d.LastSeen = &value
		}

		devices = append(devices, d)
	}

	writeDeviceJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"total":   len(devices),
		"devices": devices,
	})
}

func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeDeviceJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "ERROR",
			"message": "Method tidak diizinkan",
		})
		return
	}

	var input struct {
		Name        string `json:"name"`
		DeviceType  string `json:"device_type"`
		Vendor      string `json:"vendor"`
		Model       string `json:"model"`
		IPAddress   string `json:"ip_address"`
		MACAddress  string `json:"mac_address"`
		APIEndpoint string `json:"api_endpoint"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeDeviceJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "JSON tidak valid",
		})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.DeviceType = strings.TrimSpace(input.DeviceType)
	input.Vendor = strings.TrimSpace(input.Vendor)
	input.Model = strings.TrimSpace(input.Model)
	input.IPAddress = strings.TrimSpace(input.IPAddress)
	input.MACAddress = strings.TrimSpace(input.MACAddress)
	input.APIEndpoint = strings.TrimSpace(input.APIEndpoint)

	if input.Name == "" || input.DeviceType == "" {
		writeDeviceJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "Nama dan jenis perangkat wajib diisi",
		})
		return
	}

	var id int64
	err := h.DB.QueryRow(`
                INSERT INTO devices
                (name, device_type, vendor, model, ip_address,
                 mac_address, api_endpoint, status)
                VALUES ($1, $2, $3, $4, $5, $6, $7, 'UNKNOWN')
                RETURNING id
        `,
		input.Name,
		input.DeviceType,
		input.Vendor,
		input.Model,
		input.IPAddress,
		input.MACAddress,
		input.APIEndpoint,
	).Scan(&id)

	if err != nil {
		writeDeviceJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	writeDeviceJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "OK",
		"message": "Perangkat berhasil didaftarkan",
		"id":      id,
	})
}

func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeDeviceJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "ERROR",
			"message": "Method tidak diizinkan",
		})
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/admin/devices/")
	idText = strings.TrimSpace(idText)

	id, err := strconv.Atoi(idText)
	if err != nil || id <= 0 {
		writeDeviceJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "ID perangkat tidak valid",
		})
		return
	}

	result, err := h.DB.Exec(
		"DELETE FROM devices WHERE id = $1",
		id,
	)

	if err != nil {
		writeDeviceJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	affected, _ := result.RowsAffected()

	if affected == 0 {
		writeDeviceJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "ERROR",
			"message": "Perangkat tidak ditemukan",
		})
		return
	}

	writeDeviceJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"message": "Perangkat berhasil dihapus",
	})
}

func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		writeDeviceJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "ERROR",
			"message": "Method tidak diizinkan",
		})
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/admin/devices/")
	idText = strings.TrimSpace(idText)

	id, err := strconv.Atoi(idText)
	if err != nil || id <= 0 {
		writeDeviceJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "ID perangkat tidak valid",
		})
		return
	}

	var input struct {
		Name        string `json:"name"`
		DeviceType  string `json:"device_type"`
		Vendor      string `json:"vendor"`
		Model       string `json:"model"`
		IPAddress   string `json:"ip_address"`
		MACAddress  string `json:"mac_address"`
		APIEndpoint string `json:"api_endpoint"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeDeviceJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "JSON tidak valid",
		})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.DeviceType = strings.TrimSpace(input.DeviceType)
	input.Vendor = strings.TrimSpace(input.Vendor)
	input.Model = strings.TrimSpace(input.Model)
	input.IPAddress = strings.TrimSpace(input.IPAddress)
	input.MACAddress = strings.TrimSpace(input.MACAddress)
	input.APIEndpoint = strings.TrimSpace(input.APIEndpoint)

	if input.Name == "" || input.DeviceType == "" {
		writeDeviceJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "Nama dan jenis perangkat wajib diisi",
		})
		return
	}

	result, err := h.DB.Exec(`
		UPDATE devices
            SET name = $1,
                device_type = $2,
                vendor = $3,
                model = $4,
                ip_address = $5,
                mac_address = $6,
                api_endpoint = $7
            WHERE id = $8
	`,
		input.Name,
		input.DeviceType,
		input.Vendor,
		input.Model,
		input.IPAddress,
		input.MACAddress,
		input.APIEndpoint,
		id,
	)

	if err != nil {
		writeDeviceJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	affected, _ := result.RowsAffected()

	if affected == 0 {
		writeDeviceJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "ERROR",
			"message": "Perangkat tidak ditemukan",
		})
		return
	}

	writeDeviceJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"message": "Perangkat berhasil diperbarui",
		"id":      id,
	})
}
