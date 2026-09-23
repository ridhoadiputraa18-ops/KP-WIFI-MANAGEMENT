package handlers

import (
	"database/sql"
	"net/http"
)

type TechnicianDeviceHandler struct {
	DB *sql.DB
}

func NewTechnicianDeviceHandler(db *sql.DB) *TechnicianDeviceHandler {
	return &TechnicianDeviceHandler{DB: db}
}

func (h *TechnicianDeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			id,
			name,
			device_type,
			COALESCE(vendor, ''),
			COALESCE(model, ''),
			COALESCE(ip_address, ''),
			COALESCE(mac_address, ''),
			status,
			COALESCE(last_seen, ''),
			created_at
		FROM devices
		ORDER BY id DESC
	`)
	if err != nil {
		http.Error(w, "gagal mengambil perangkat", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	devices := []map[string]interface{}{}

	for rows.Next() {
		var (
			id         int
			name       string
			deviceType string
			vendor     string
			model      string
			ipAddress  string
			macAddress string
			status     string
			lastSeen   string
			createdAt  string
		)

		if err := rows.Scan(
			&id,
			&name,
			&deviceType,
			&vendor,
			&model,
			&ipAddress,
			&macAddress,
			&status,
			&lastSeen,
			&createdAt,
		); err != nil {
			http.Error(w, "gagal membaca data perangkat", http.StatusInternalServerError)
			return
		}

		devices = append(devices, map[string]interface{}{
			"id":          id,
			"name":        name,
			"device_type": deviceType,
			"vendor":      vendor,
			"model":       model,
			"ip_address":  ipAddress,
			"mac_address": macAddress,
			"status":      status,
			"last_seen":   lastSeen,
			"created_at":  createdAt,
		})
	}

	writeDeviceJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"total":   len(devices),
		"devices": devices,
	})
}
