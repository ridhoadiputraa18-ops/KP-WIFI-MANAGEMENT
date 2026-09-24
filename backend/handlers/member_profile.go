package handlers

import (
	"database/sql"
	"net/http"
)

type MemberProfileHandler struct {
	DB *sql.DB
}

func NewMemberProfileHandler(db *sql.DB) *MemberProfileHandler {
	return &MemberProfileHandler{DB: db}
}

func (h *MemberProfileHandler) Get(w http.ResponseWriter, r *http.Request, userID int) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	var profile struct {
		ID          int    `json:"id"`
		UserID      int    `json:"user_id"`
		Username    string `json:"username"`
		FullName    string `json:"full_name"`
		Email       string `json:"email"`
		MemberCode  string `json:"member_code"`
		Status      string `json:"status"`
		AccessStart string `json:"access_start"`
		AccessEnd   string `json:"access_end"`
	}

	err := h.DB.QueryRow(`
		SELECT
			m.id,
			u.id,
			u.username,
			u.full_name,
			COALESCE(u.email, ''),
			m.member_code,
			m.status,
			COALESCE(m.access_start, ''),
			COALESCE(m.access_end, '')
		FROM members m
		JOIN users u ON u.id = m.user_id
		WHERE u.id = $1
	`, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Username,
		&profile.FullName,
		&profile.Email,
		&profile.MemberCode,
		&profile.Status,
		&profile.AccessStart,
		&profile.AccessEnd,
	)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status": "MEMBER_PROFILE_NOT_FOUND",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"profile": profile,
	})
}
