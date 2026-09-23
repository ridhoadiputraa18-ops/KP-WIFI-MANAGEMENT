package handlers

import (
	"database/sql"
)

type MemberExpiryHandler struct {
	DB *sql.DB
}

func NewMemberExpiryHandler(db *sql.DB) *MemberExpiryHandler {
	return &MemberExpiryHandler{DB: db}
}

func (h *MemberExpiryHandler) ExpireMembers() error {
	_, err := h.DB.Exec(`
		UPDATE members
		SET status = 'EXPIRED'
		WHERE status = 'ACTIVE'
		  AND access_end IS NOT NULL
		  AND access_end != ''
		  AND datetime(access_end) <= CURRENT_TIMESTAMP
	`)
	return err
}
