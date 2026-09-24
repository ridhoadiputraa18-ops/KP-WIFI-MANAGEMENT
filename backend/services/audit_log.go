package services

import (
	"database/sql"
	"net/http"
)

type AuditLogger struct {
	DB *sql.DB
}

func NewAuditLogger(db *sql.DB) *AuditLogger {
	return &AuditLogger{DB: db}
}

func (a *AuditLogger) Log(
	r *http.Request,
	userID *int64,
	action string,
	targetType string,
	targetID *int64,
	description string,
) error {
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.Header.Get("X-Real-IP")
	}
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}

	_, err := a.DB.Exec(`
		INSERT INTO audit_logs
			(user_id, action, target_type, target_id, description, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		userID,
		action,
		targetType,
		targetID,
		description,
		ipAddress,
	)

	return err
}
