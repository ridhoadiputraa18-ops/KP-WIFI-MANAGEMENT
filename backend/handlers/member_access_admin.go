package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kp-wifi-management/middleware"
	"kp-wifi-management/services"
)

type MemberAccessAdminHandler struct {
	DB *sql.DB
}

func NewMemberAccessAdminHandler(db *sql.DB) *MemberAccessAdminHandler {
	return &MemberAccessAdminHandler{DB: db}
}

type UpdateMemberAccessRequest struct {
	AccessStart string `json:"access_start"`
	AccessEnd   string `json:"access_end"`
	Status      string `json:"status"`
}

func (h *MemberAccessAdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"status": "METHOD_NOT_ALLOWED",
		})
		return
	}

	const prefix = "/api/admin/members/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	path = strings.TrimSuffix(path, "/")

	parts := strings.Split(path, "/")

	if len(parts) != 2 || parts[1] != "access" {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status": "NOT_FOUND",
		})
		return
	}

	memberID, err := strconv.Atoi(parts[0])
	if err != nil || memberID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_MEMBER_ID",
		})
		return
	}

	var req UpdateMemberAccessRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_JSON",
		})
		return
	}

	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))

	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	if req.Status != "ACTIVE" && req.Status != "INACTIVE" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "INVALID_STATUS",
		})
		return
	}

	var accessStart interface{}
	var accessEnd interface{}

	if strings.TrimSpace(req.AccessStart) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(req.AccessStart))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_ACCESS_START",
				"message": "access_start harus RFC3339",
			})
			return
		}
		accessStart = t.UTC().Format(time.RFC3339)
	}

	if strings.TrimSpace(req.AccessEnd) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(req.AccessEnd))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_ACCESS_END",
				"message": "access_end harus RFC3339",
			})
			return
		}
		accessEnd = t.UTC().Format(time.RFC3339)
	}

	if accessStart != nil && accessEnd != nil {
		start := accessStart.(string)
		end := accessEnd.(string)

		startTime, _ := time.Parse(time.RFC3339, start)
		endTime, _ := time.Parse(time.RFC3339, end)

		if !endTime.After(startTime) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"status":  "INVALID_ACCESS_RANGE",
				"message": "access_end harus setelah access_start",
			})
			return
		}
	}

	result, err := h.DB.Exec(`
		UPDATE members
            SET access_start = $1,
                access_end = $2,
                status = $3
            WHERE id = $4
	`, accessStart, accessEnd, req.Status, memberID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "DATABASE_ERROR",
		})
		return
	}

	if rows == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status": "MEMBER_NOT_FOUND",
		})
		return
	}

	var memberUsername string
	var memberFullName string

	err = h.DB.QueryRow(`
		SELECT u.username, COALESCE(u.full_name, '')
		FROM members m
		JOIN users u ON u.id = m.user_id
		WHERE m.id = $1
	`, memberID).Scan(&memberUsername, &memberFullName)

	if err != nil {
		fmt.Printf("member lookup audit error: %v\n", err)
		memberUsername = fmt.Sprintf("member_id=%d", memberID)
	}

	if memberFullName == "" {
		memberFullName = memberUsername
	}

	auditLogger := services.NewAuditLogger(h.DB)

	var adminUserID *int64

	if adminUser, ok := middleware.CurrentUser(r); ok {
		id := int64(adminUser.ID)
		adminUserID = &id
	}

	description := fmt.Sprintf(
		"Admin memperbarui Hak Akses Member: %s (%s) menjadi %s",
		memberFullName,
		memberUsername,
		req.Status,
	)

	targetID := int64(memberID)

	if err := auditLogger.Log(
		r,
		adminUserID,
		"MEMBER_ACCESS_UPDATE",
		"MEMBER",
		&targetID,
		description,
	); err != nil {
		fmt.Printf("audit log error: %v\n", err)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "OK",
		"message":   "Akses Member berhasil diperbarui",
		"member_id": memberID,
		"access": map[string]interface{}{
			"access_start": accessStart,
			"access_end":   accessEnd,
			"status":       req.Status,
		},
	})
}
