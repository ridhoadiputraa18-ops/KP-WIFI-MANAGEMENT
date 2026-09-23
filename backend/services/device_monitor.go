package services

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"
)

type DeviceMonitor struct {
	DB *sql.DB
}

type DeviceMonitorResult struct {
	DeviceID int64
	Status   string
	LastSeen *time.Time
	Error    string
}

type monitorDevice struct {
	ID          int64
	IP          string
	APIEndpoint string
}

func NewDeviceMonitor(db *sql.DB) *DeviceMonitor {
	return &DeviceMonitor{DB: db}
}

func (m *DeviceMonitor) CheckAll(ctx context.Context) ([]DeviceMonitorResult, error) {
	rows, err := m.DB.QueryContext(ctx, `
		SELECT id, COALESCE(ip_address, ''), COALESCE(api_endpoint, '')
		FROM devices
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []monitorDevice

	for rows.Next() {
		var d monitorDevice
		if err := rows.Scan(&d.ID, &d.IP, &d.APIEndpoint); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	results := make([]DeviceMonitorResult, 0, len(devices))

	for _, device := range devices {
		result := m.checkDevice(ctx, device)

		if err := m.updateDeviceStatus(ctx, result); err != nil {
			result.Error = err.Error()
		}

		results = append(results, result)
	}

	return results, nil
}

func (m *DeviceMonitor) checkDevice(ctx context.Context, device monitorDevice) DeviceMonitorResult {
	result := DeviceMonitorResult{
		DeviceID: device.ID,
		Status:   "UNKNOWN",
	}

	target := strings.TrimSpace(device.IP)

	if target == "" {
		return result
	}

	if !strings.Contains(target, ":") {
		target = net.JoinHostPort(target, "80")
	}

	dialer := net.Dialer{
		Timeout: 3 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		result.Status = "OFFLINE"
		result.Error = fmt.Sprintf("koneksi gagal: %v", err)
		return result
	}

	_ = conn.Close()

	now := time.Now().UTC()
	result.Status = "ONLINE"
	result.LastSeen = &now

	return result
}

func (m *DeviceMonitor) updateDeviceStatus(
	ctx context.Context,
	result DeviceMonitorResult,
) error {
	if result.LastSeen != nil {
		_, err := m.DB.ExecContext(ctx, `
			UPDATE devices
			SET status = ?, last_seen = ?
			WHERE id = ?
		`,
			result.Status,
			result.LastSeen,
			result.DeviceID,
		)
		return err
	}

	_, err := m.DB.ExecContext(ctx, `
		UPDATE devices
		SET status = ?
		WHERE id = ?
	`,
		result.Status,
		result.DeviceID,
	)

	return err
}
