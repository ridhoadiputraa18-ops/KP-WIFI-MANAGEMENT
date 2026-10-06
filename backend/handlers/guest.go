package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"kp-wifi-management/middleware"
	"kp-wifi-management/services"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type GuestHandler struct {
	DB *sql.DB
}

func NewGuestHandler(db *sql.DB) *GuestHandler {
	return &GuestHandler{DB: db}
}

func writeGuestJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// POST /api/guest/login
// Login khusus Guest Portal.
// Akun Guest terpisah dari akun ADMIN/MEMBER/TECHNICIAN.
func (h *GuestHandler) Login(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Method tidak diizinkan",
                })
                return
        }

        var req struct {
                Username string `json:"username"`
                Password string `json:"password"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "JSON tidak valid",
                })
                return
        }

        req.Username = strings.TrimSpace(req.Username)

        if req.Username == "" || req.Password == "" {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Username dan password wajib diisi",
                })
                return
        }

        var (
                guestID     int64
                passwordHash string
                fullName    string
                status      string
        )

        err := h.DB.QueryRow(`
                SELECT id, password_hash, full_name, status
                FROM guest_accounts
                WHERE username = $1
                LIMIT 1
        `, req.Username).Scan(
                &guestID,
                &passwordHash,
                &fullName,
                &status,
        )

        if err == sql.ErrNoRows {
                writeGuestJSON(w, http.StatusUnauthorized, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Username atau password Guest salah",
                })
                return
        }

        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Gagal memeriksa akun Guest",
                })
                return
        }

        if !VerifyPassword(req.Password, passwordHash) {
                writeGuestJSON(w, http.StatusUnauthorized, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Username atau password Guest salah",
                })
                return
        }

        if strings.ToUpper(status) != "ACTIVE" {
                writeGuestJSON(w, http.StatusForbidden, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Akun Guest tidak aktif",
                })
                return
        }

        token, err := GenerateToken()
        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Gagal membuat sesi Guest",
                })
                return
        }

        expires := time.Now().UTC().Add(8 * time.Hour)

        _, err = h.DB.Exec(`
                INSERT INTO guest_auth_sessions
                        (token, guest_account_id, expires_at)
                VALUES
                        ($1, $2, $3)
        `, token, guestID, expires)

        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Gagal menyimpan sesi Guest",
                })
                return
        }

        http.SetCookie(w, &http.Cookie{
                Name:     "guest_portal_session",
                Value:    token,
                Path:     "/",
                HttpOnly: true,
                Secure:   r.TLS != nil,
                SameSite: http.SameSiteLaxMode,
                Expires:  expires,
                MaxAge:   8 * 60 * 60,
        })

        writeGuestJSON(w, http.StatusOK, map[string]interface{}{
                "status":  "OK",
                "message": "Login Guest berhasil",
                "guest": map[string]interface{}{
                        "id":        guestID,
                        "username":  req.Username,
                        "full_name": fullName,
                },
        })
}

// GET /api/guest/config
// Dipakai portal Guest untuk mengetahui SSID dan durasi akses.
// Password Wi-Fi TIDAK dikirim ke browser.
func (h *GuestHandler) Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status":  "ERROR",
			"message": "Method tidak diizinkan",
		})
		return
	}

	var id int
	var ssid, networkType, status string
	var duration int

	err := h.DB.QueryRow(`
		SELECT id, ssid, network_type, guest_duration_minutes, status
		FROM wifi_configs
		WHERE network_type = 'GUEST'
		  AND status = 'ACTIVE'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&id, &ssid, &networkType, &duration, &status)

	if err == sql.ErrNoRows {
		writeGuestJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "ERROR",
			"message": "Konfigurasi Guest Wi-Fi belum tersedia",
		})
		return
	}

	if err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	writeGuestJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"config": map[string]interface{}{
			"id":             id,
			"ssid":           ssid,
			"network_type":   networkType,
			"guest_duration": duration,
			"config_status":  status,
		},
	})
}

// POST /api/guest/session
// Membuat data Guest + session.
// Ini mencatat sesi aplikasi; enforcement jaringan akan kita integrasikan
// ke gateway/MikroTik pada tahap berikutnya.
func (h *GuestHandler) StartSession(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
                        "status": "ERROR",
                        "message": "Method tidak diizinkan",
                })
                return
        }

        var req struct {
                Name      string `json:"name"`
                Company   string `json:"company"`
                Purpose   string `json:"purpose"`
                ClientIP  string `json:"client_ip"`
                ClientMAC string `json:"client_mac"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status": "ERROR",
                        "message": "JSON tidak valid",
                })
                return
        }

        req.Name = strings.TrimSpace(req.Name)
        req.Company = strings.TrimSpace(req.Company)
        req.Purpose = strings.TrimSpace(req.Purpose)
        req.ClientIP = strings.TrimSpace(req.ClientIP)
        req.ClientMAC = strings.TrimSpace(req.ClientMAC)

        if req.Name == "" {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status": "ERROR",
                        "message": "Atas Nama wajib diisi",
                })
                return
        }

        if req.Company == "" {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status": "ERROR",
                        "message": "PT/Instansi wajib diisi",
                })
                return
        }

        if req.Purpose == "" {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status": "ERROR",
                        "message": "Keperluan wajib diisi",
                })
                return
        }

        if req.ClientIP != "" && net.ParseIP(req.ClientIP) == nil {
                writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
                        "status": "ERROR",
                        "message": "IP client tidak valid",
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
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status": "ERROR",
                        "message": err.Error(),
                })
                return
        }

        if duration <= 0 {
                duration = 120
        }

        now := time.Now().UTC()
        end := now.Add(time.Duration(duration) * time.Minute)

        tx, err := h.DB.Begin()
        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status": "ERROR",
                        "message": err.Error(),
                })
                return
        }
        defer tx.Rollback()

        var guestID int64

        err = tx.QueryRow(`
                INSERT INTO guests
                        (name, company, purpose, guest_account_id,
                         access_start, access_end, status)
                VALUES
                        ($1, $2, $3, NULL, $4, $5, 'ACTIVE')
                RETURNING id
        `, req.Name, req.Company, req.Purpose, now, end).Scan(&guestID)

        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status": "ERROR",
                        "message": err.Error(),
                })
                return
        }

        var sessionID int64

        err = tx.QueryRow(`
                INSERT INTO sessions
                        (guest_id, client_ip, client_mac, started_at, status)
                VALUES
                        ($1, $2, $3, $4, 'ONLINE')
                RETURNING id
        `, guestID, req.ClientIP, req.ClientMAC, now).Scan(&sessionID)

        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status": "ERROR",
                        "message": err.Error(),
                })
                return
        }

        if err := tx.Commit(); err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status": "ERROR",
                        "message": err.Error(),
                })
                return
        }

        writeGuestJSON(w, http.StatusCreated, map[string]interface{}{
                "status": "OK",
                "message": "Data Guest berhasil disimpan dan sesi Wi-Fi dimulai",
                "guest": map[string]interface{}{
                        "id":           guestID,
                        "name":         req.Name,
                        "company":      req.Company,
                        "purpose":      req.Purpose,
                        "access_start": now.Format(time.RFC3339),
                        "access_end":   end.Format(time.RFC3339),
                        "duration":     duration,
                },
                "session": map[string]interface{}{
                        "id":         sessionID,
                        "status":     "ONLINE",
                        "client_ip":  req.ClientIP,
                        "client_mac": req.ClientMAC,
                },
        })
}

// GET /api/admin/guests
func (h *GuestHandler) ExpireSessions() error {
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
        UPDATE sessions
        SET status = 'EXPIRED',
            ended_at = COALESCE(ended_at, (
                SELECT g.access_end
                FROM guests g
                WHERE g.id = sessions.guest_id
            ))
        WHERE guest_id IS NOT NULL
          AND status = 'ONLINE'
          AND EXISTS (
              SELECT 1
              FROM guests g
              WHERE g.id = sessions.guest_id
                AND g.access_end IS NOT NULL
                AND g.access_end <= CURRENT_TIMESTAMP
          )
    `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
        UPDATE guests
        SET status = 'EXPIRED'
        WHERE status = 'ACTIVE'
          AND access_end IS NOT NULL
          AND access_end <= CURRENT_TIMESTAMP
    `)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (h *GuestHandler) AdminList(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
                writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
                        "status": "ERROR",
                })
                return
        }

        if err := h.ExpireSessions(); err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status":  "ERROR",
                        "message": "Gagal memperbarui status session Guest",
                })
                return
        }

        rows, err := h.DB.Query(`
                SELECT
                        g.id,
                        g.name,
                        COALESCE(g.company, ''),
                        COALESCE(g.purpose, ''),
                        COALESCE(CAST(g.access_start AS TEXT), ''),
                        COALESCE(CAST(g.access_end AS TEXT), ''),
                        g.status,
                        COALESCE(s.id, 0),
                        COALESCE(s.client_ip, ''),
                        COALESCE(s.client_mac, ''),
                        COALESCE(s.status, '')
                FROM guests g
                LEFT JOIN sessions s ON s.guest_id = g.id
                ORDER BY g.id DESC
        `)

        if err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status":  "ERROR",
                        "message": err.Error(),
                })
                return
        }
        defer rows.Close()

        type Guest struct {
                ID          int    `json:"id"`
                Name        string `json:"name"`
                Company     string `json:"company"`
                Purpose     string `json:"purpose"`
                AccessStart string `json:"access_start"`
                AccessEnd   string `json:"access_end"`
                Status      string `json:"status"`
                SessionID   int    `json:"session_id"`
                ClientIP    string `json:"client_ip"`
                ClientMAC   string `json:"client_mac"`
                SessionStat string `json:"session_status"`
        }

        guests := make([]Guest, 0)

        for rows.Next() {
                var g Guest

                err := rows.Scan(
                        &g.ID,
                        &g.Name,
                        &g.Company,
                        &g.Purpose,
                        &g.AccessStart,
                        &g.AccessEnd,
                        &g.Status,
                        &g.SessionID,
                        &g.ClientIP,
                        &g.ClientMAC,
                        &g.SessionStat,
                )

                if err != nil {
                        writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                                "status":  "ERROR",
                                "message": err.Error(),
                        })
                        return
                }

                guests = append(guests, g)
        }

        if err := rows.Err(); err != nil {
                writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
                        "status":  "ERROR",
                        "message": err.Error(),
                })
                return
        }

        writeGuestJSON(w, http.StatusOK, map[string]interface{}{
                "status": "OK",
                "data":   guests,
        })
}

func (h *GuestHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status": "ERROR",
		})
		return
	}

	var req struct {
		SSID           string `json:"ssid"`
		Password       string `json:"password"`
		DurationMinute int    `json:"duration_minutes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "JSON tidak valid",
		})
		return
	}

	req.SSID = strings.TrimSpace(req.SSID)
	req.Password = strings.TrimSpace(req.Password)

	if req.SSID == "" {
		writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "SSID wajib diisi",
		})
		return
	}

	if req.DurationMinute <= 0 {
		req.DurationMinute = 120
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}
	defer tx.Rollback()

	var configID int64
	var existingPassword string

	err = tx.QueryRow(`
		SELECT id, password_encrypted
		FROM wifi_configs
		WHERE network_type = 'GUEST'
		ORDER BY id ASC
		LIMIT 1
	`).Scan(&configID, &existingPassword)

	if err == sql.ErrNoRows {
		if len(req.Password) < 8 {
			writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
				"status":  "ERROR",
				"message": "Password Wi-Fi minimal 8 karakter",
			})
			return
		}

		encryptedPassword, err := encryptPassword(req.Password)
		if err != nil {
			writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"status":  "ERROR",
				"message": "Gagal mengenkripsi password Wi-Fi",
			})
			return
		}

		err = tx.QueryRow(`
                        INSERT INTO wifi_configs
                                (ssid, network_type, password_encrypted,
                                 guest_duration_minutes, status)
                        VALUES ($1, 'GUEST', $2, $3, 'ACTIVE')
                        RETURNING id
                `, req.SSID, encryptedPassword, req.DurationMinute).Scan(&configID)

		if err != nil {
			writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"status":  "ERROR",
				"message": err.Error(),
			})
			return
		}

	} else if err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	} else {
		// Password kosong berarti pertahankan password lama.
		passwordToSave := existingPassword

		if req.Password != "" {
			if len(req.Password) < 8 {
				writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
					"status":  "ERROR",
					"message": "Password Wi-Fi minimal 8 karakter",
				})
				return
			}

			passwordToSave, err = encryptPassword(req.Password)
			if err != nil {
				writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
					"status":  "ERROR",
					"message": "Gagal mengenkripsi password Wi-Fi",
				})
				return
			}
		}

		_, err = tx.Exec(`
                        UPDATE wifi_configs
                        SET ssid = $1,
                                password_encrypted = $2,
                                guest_duration_minutes = $3,
                                status = 'ACTIVE',
                                updated_at = CURRENT_TIMESTAMP
                        WHERE id = $4
                `, req.SSID, passwordToSave, req.DurationMinute, configID)

		if err != nil {
			writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"status":  "ERROR",
				"message": err.Error(),
			})
			return
		}
	}

	if _, err = tx.Exec(`
                UPDATE wifi_configs
                SET status = 'INACTIVE'
                WHERE network_type = 'GUEST'
                  AND id != $1
        `, configID); err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	if err = tx.Commit(); err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	currentUser, _ := middleware.CurrentUser(r)

	if currentUser != nil {
		userID := int64(currentUser.ID)
		auditLogger := services.NewAuditLogger(h.DB)

		if err := auditLogger.Log(
			r,
			&userID,
			"GUEST_WIFI_CONFIG_UPDATE",
			"WIFI_CONFIG",
			&configID,
			"Admin memperbarui konfigurasi Guest Wi-Fi: SSID="+req.SSID+
				", durasi="+strconv.Itoa(req.DurationMinute)+" menit",
		); err != nil {
			fmt.Printf("audit guest config error: %v\n", err)
		}
	}

	writeGuestJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "OK",
		"message": "Konfigurasi Guest Wi-Fi tersimpan",
		"config": map[string]interface{}{
			"ssid":                   req.SSID,
			"guest_duration_minutes": req.DurationMinute,
			"status":                 "ACTIVE",
		},
	})
}
func (h *GuestHandler) AdminConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status": "ERROR",
		})
		return
	}

	var id, duration int
	var ssid, networkType, status string

	err := h.DB.QueryRow(`
		SELECT id, ssid, network_type, guest_duration_minutes, status
		FROM wifi_configs
		WHERE network_type = 'GUEST'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&id, &ssid, &networkType, &duration, &status)

	if err == sql.ErrNoRows {
		writeGuestJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "ERROR",
			"message": "Belum ada konfigurasi Guest Wi-Fi",
		})
		return
	}

	if err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	writeGuestJSON(w, http.StatusOK, map[string]interface{}{
		"status": "OK",
		"config": map[string]interface{}{
			"id":                     id,
			"ssid":                   ssid,
			"network_type":           networkType,
			"guest_duration_minutes": duration,
			"status":                 status,
		},
	})
}

// POST /api/admin/guests/{id}/stop
func (h *GuestHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGuestJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status": "ERROR",
		})
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/admin/guests/")
	idText = strings.TrimSuffix(idText, "/stop")

	guestID, err := strconv.Atoi(idText)
	if err != nil || guestID <= 0 {
		writeGuestJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  "ERROR",
			"message": "ID Guest tidak valid",
		})
		return
	}

	now := time.Now().UTC()

	result, err := h.DB.Exec(`
                UPDATE sessions
                SET status = 'OFFLINE',
                    ended_at = $1
                WHERE guest_id = $2
                  AND status = 'ONLINE'
        `, now, guestID)

	if err != nil {
		writeGuestJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "ERROR",
			"message": err.Error(),
		})
		return
	}

	affected, _ := result.RowsAffected()
	// Catat aktivitas Admin ke Audit Log.
	if user, ok := middleware.CurrentUser(r); ok {
		auditLogger := services.NewAuditLogger(h.DB)
		targetID := int64(guestID)

		description := fmt.Sprintf(
			"Admin menghentikan sesi Guest ID %d (session offline: %d)",
			guestID,
			affected,
		)

		if err := auditLogger.Log(
			r,
			func() *int64 { id := int64(user.ID); return &id }(),
			"GUEST_SESSION_STOP",
			"GUEST",
			&targetID,
			description,
		); err != nil {
			fmt.Printf("audit guest stop error: %v\n", err)
		}
	}

	_, _ = h.DB.Exec(`
                UPDATE guests
                SET status = 'EXPIRED',
                    access_end = $1
                WHERE id = $2
        `, now, guestID)

	writeGuestJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "OK",
		"message":          "Sesi Guest dihentikan",
		"guest_id":         guestID,
		"stopped_sessions": affected,
	})
}
