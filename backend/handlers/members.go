package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MemberHandler struct {
	DB *sql.DB
}

func NewMemberHandler(db *sql.DB) *MemberHandler {
	return &MemberHandler{DB: db}
}

type CreateMemberRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	MemberCode  string `json:"member_code"`
	AccessStart string `json:"access_start"`
	AccessEnd   string `json:"access_end"`
}

func (h *MemberHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	var req CreateMemberRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_JSON",
		})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)
	req.MemberCode = strings.TrimSpace(req.MemberCode)

	if req.Username == "" ||
		req.Password == "" ||
		req.FullName == "" ||
		req.MemberCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "VALIDATION_ERROR",
			"message": "username, password, full_name, dan member_code wajib diisi",
		})
		return
	}

	passwordHash, err := GeneratePassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "PASSWORD_HASH_ERROR",
		})
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}
	defer tx.Rollback()

	var roleID int

	err = tx.QueryRow(
		`SELECT id FROM roles WHERE name = 'MEMBER'`,
	).Scan(&roleID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "ROLE_NOT_FOUND",
			"message": "Role MEMBER tidak ditemukan",
		})
		return
	}

	var accessStart interface{}
	var accessEnd interface{}

	if req.AccessStart != "" {
		t, err := time.Parse(time.RFC3339, req.AccessStart)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_ACCESS_START",
				"message": "access_start harus format RFC3339",
			})
			return
		}
		accessStart = t
	}

	if req.AccessEnd != "" {
		t, err := time.Parse(time.RFC3339, req.AccessEnd)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_ACCESS_END",
				"message": "access_end harus format RFC3339",
			})
			return
		}
		accessEnd = t
	}

	var userID64 int64

	err = tx.QueryRow(`
                INSERT INTO users
                        (role_id, username, password_hash, full_name, email, status)
                VALUES ($1, $2, $3, $4, $5, 'ACTIVE')
                RETURNING id
        `,
		roleID,
		req.Username,
		passwordHash,
		req.FullName,
		nullString(req.Email),
	).Scan(&userID64)

	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "USER_CREATE_FAILED",
			"message": err.Error(),
		})
		return
	}

	_, err = tx.Exec(`
		INSERT INTO members
			(user_id, member_code, access_start, access_end, status)
		VALUES ($1, $2, $3, $4, 'ACTIVE')
	`,
		userID64,
		req.MemberCode,
		accessStart,
		accessEnd,
	)

	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "MEMBER_CREATE_FAILED",
			"message": err.Error(),
		})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":      "OK",
		"message":     "Member berhasil dibuat",
		"user_id":     userID64,
		"member_code": req.MemberCode,
	})
}

func (h *MemberHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	rows, err := h.DB.Query(`
		SELECT
			m.id,
			u.id,
			u.username,
			u.full_name,
			COALESCE(u.email, ''),
			m.member_code,
			COALESCE(m.access_start, ''),
			COALESCE(m.access_end, ''),
			m.status,
			u.created_at
		FROM members m
		JOIN users u ON u.id = m.user_id
		ORDER BY m.id DESC
	`)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}
	defer rows.Close()

	type Member struct {
		ID          int    `json:"id"`
		UserID      int    `json:"user_id"`
		Username    string `json:"username"`
		FullName    string `json:"full_name"`
		Email       string `json:"email"`
		MemberCode  string `json:"member_code"`
		AccessStart string `json:"access_start"`
		AccessEnd   string `json:"access_end"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
	}

	members := make([]Member, 0)

	for rows.Next() {
		var m Member

		if err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.Username,
			&m.FullName,
			&m.Email,
			&m.MemberCode,
			&m.AccessStart,
			&m.AccessEnd,
			&m.Status,
			&m.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"status": "DATABASE_ERROR",
			})
			return
		}

		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"members": members,
		"total":   len(members),
	})
}

func (h *MemberHandler) Detail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/admin/members/")

	id, err := strconv.Atoi(idText)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_ID",
		})
		return
	}

	var member struct {
		ID          int    `json:"id"`
		UserID      int    `json:"user_id"`
		Username    string `json:"username"`
		FullName    string `json:"full_name"`
		Email       string `json:"email"`
		MemberCode  string `json:"member_code"`
		AccessStart string `json:"access_start"`
		AccessEnd   string `json:"access_end"`
		Status      string `json:"status"`
	}

	err = h.DB.QueryRow(`
		SELECT
			m.id,
			u.id,
			u.username,
			u.full_name,
			COALESCE(u.email, ''),
			m.member_code,
			COALESCE(m.access_start, ''),
			COALESCE(m.access_end, ''),
			m.status
		FROM members m
		JOIN users u ON u.id = m.user_id
		WHERE m.id = $1
	`, id).Scan(
		&member.ID,
		&member.UserID,
		&member.Username,
		&member.FullName,
		&member.Email,
		&member.MemberCode,
		&member.AccessStart,
		&member.AccessEnd,
		&member.Status,
	)

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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"member": member,
	})
}

func nullString(value string) interface{} {
	if value == "" {
		return nil
	}

	return value
}
