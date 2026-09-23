package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"kp-wifi-management/middleware"
	"kp-wifi-management/services"
)

type TechnicianMonitorHandler struct {
	Monitor *services.DeviceMonitor
	DB      *sql.DB
}

func NewTechnicianMonitorHandler(db *sql.DB) *TechnicianMonitorHandler {
	return &TechnicianMonitorHandler{
		Monitor: services.NewDeviceMonitor(db),
		DB:      db,
	}
}

func (h *TechnicianMonitorHandler) Check(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"status":"METHOD_NOT_ALLOWED","message":"Method tidak diizinkan"}`, http.StatusMethodNotAllowed)
		return
	}

	if _, ok := middleware.CurrentUser(r); !ok {
		http.Error(w, `{"status":"UNAUTHORIZED","message":"Silakan login terlebih dahulu"}`, http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results, err := h.Monitor.CheckAll(ctx)
	if err != nil {
		http.Error(w, `{"status":"ERROR","message":"Gagal menjalankan monitoring"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":  "OK",
		"total":   len(results),
		"results": results,
	}

	writeJSON(w, http.StatusOK, response)
}
