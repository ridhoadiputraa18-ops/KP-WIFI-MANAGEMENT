package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kp-wifi-management/database"
	"kp-wifi-management/handlers"
	"kp-wifi-management/middleware"
	"kp-wifi-management/services"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func main() {
	db := database.Open()
	defer db.Close()

	database.Init(db)

	authHandler := handlers.NewAuthHandler(db)
	memberAPI := handlers.NewMemberHandler(db)
	memberProfileAPI := handlers.NewMemberProfileHandler(db)
	memberAccessAPI := handlers.NewMemberAccessHandler(db)
	memberAccessAdminAPI := handlers.NewMemberAccessAdminHandler(db)
	memberSessionAPI := handlers.NewMemberSessionHandler(db)
	memberActiveSessionAPI := handlers.NewMemberActiveSessionHandler(db)
	memberSessionStopAPI := handlers.NewMemberSessionStopHandler(db)
	memberUsageAPI := handlers.NewMemberUsageHandler(db)
	adminUsageAPI := handlers.NewAdminUsageHandler(db)
	adminAuditAPI := handlers.NewAdminAuditHandler(db)
	guestAPI := handlers.NewGuestHandler(db)
	memberExpiryAPI := handlers.NewMemberExpiryHandler(db)

	// Sinkronisasi status akses saat server mulai.
	if err := memberExpiryAPI.ExpireMembers(); err != nil {
		log.Println("Gagal initial expire member:", err)
	}
	deviceAPI := handlers.NewDeviceHandler(db)
	deviceMonitor := services.NewDeviceMonitor(db)
	technicianMonitorAPI := handlers.NewTechnicianMonitorHandler(db)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			results, err := deviceMonitor.CheckAll(ctx)
			if err != nil {
				log.Printf("device monitoring error: %v", err)
			} else if len(results) > 0 {
				log.Printf("device monitoring: checked %d device(s)", len(results))
			}

			if err := memberExpiryAPI.ExpireMembers(); err != nil {
				log.Println("Gagal expire member:", err)
			}

			if err := guestAPI.ExpireSessions(); err != nil {
				log.Printf("guest session expiry error: %v", err)
			}

			cancel()
			<-ticker.C
		}
	}()
	technicianDeviceAPI := handlers.NewTechnicianDeviceHandler(db)
	technicianAdminAPI := handlers.NewTechnicianAdminHandler(db)
	technicianProfileAPI := handlers.NewTechnicianProfileHandler(db)

	auth := middleware.NewAuth(db)

	http.Handle("/", http.FileServer(http.Dir("../frontend")))
	http.Handle("/technician/", auth.RequireRole("TECHNICIAN", http.StripPrefix("/technician/", http.FileServer(http.Dir("../frontend/technician")))))

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(HealthResponse{
			Status:  "OK",
			Service: "WiFi Management API",
		})
	})

	http.HandleFunc("/api/auth/login", authHandler.Login)
	http.HandleFunc("/api/auth/logout", authHandler.Logout)

	// ==========================
	// GUEST WI-FI
	// ==========================
	http.HandleFunc("/api/guest/config", guestAPI.Config)
	http.HandleFunc("/api/guest/session", guestAPI.StartSession)

	// ==========================
	// TECHNICIAN
	// ==========================
	http.Handle("/api/technician/profile", auth.RequireRole("TECHNICIAN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			technicianProfileAPI.Profile(w, r)
			return
		}
		writeMethodNotAllowed(w)
	})))
	http.Handle("/api/technician/devices", auth.RequireRole("TECHNICIAN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			technicianDeviceAPI.List(w, r)
			return
		}
		writeMethodNotAllowed(w)
	})))
	http.Handle("/api/technician/monitor", auth.RequireRole("TECHNICIAN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		technicianMonitorAPI.Check(w, r)
	})))

	http.Handle("/api/admin/audit-logs", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminAuditAPI.List(w, r)
	})))

	http.Handle("/api/admin/technicians", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			technicianAdminAPI.List(w, r)
		case http.MethodPost:
			technicianAdminAPI.Create(w, r)
		default:
			writeMethodNotAllowed(w)
		}
	})))

	http.Handle("/api/admin/technicians/", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		technicianAdminAPI.Delete(w, r)
	})))
	http.Handle("/api/admin/devices", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			deviceAPI.List(w, r)
		case http.MethodPost:
			deviceAPI.Create(w, r)
		default:
			writeMethodNotAllowed(w)
		}
	})))

	http.Handle("/api/admin/devices/", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut, http.MethodPatch:
			deviceAPI.Update(w, r)
		case http.MethodDelete:
			deviceAPI.Delete(w, r)
		default:
			writeMethodNotAllowed(w)
		}
	})))

	http.Handle("/api/admin/guests", auth.RequireRole("ADMIN", http.HandlerFunc(guestAPI.AdminList)))
	http.Handle("/api/admin/wifi-config", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			guestAPI.AdminConfig(w, r)
			return
		}
		guestAPI.SaveConfig(w, r)
	})))
	http.Handle("/api/admin/guests/", auth.RequireRole("ADMIN", http.HandlerFunc(guestAPI.StopSession)))

	adminTestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "OK",
			"message": "Admin endpoint berhasil diakses",
		})
	})

	memberTestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "OK",
			"message": "Member endpoint berhasil diakses",
		})
	})

	technicianTestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "OK",
			"message": "Technician endpoint berhasil diakses",
		})
	})

	http.Handle(
		"/api/admin/test",
		auth.RequireRole("ADMIN", adminTestHandler),
	)

	http.Handle(
		"/api/admin/members",
		auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				memberAPI.List(w, r)
			case http.MethodPost:
				memberAPI.Create(w, r)
			default:
				writeMethodNotAllowed(w)
			}
		})),
	)

	http.Handle(
		"/api/admin/members/",
		auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut && len(r.URL.Path) > len("/api/admin/members/") {
				if len(r.URL.Path) >= 7 && r.URL.Path[len(r.URL.Path)-7:] == "/access" {
					memberAccessAdminAPI.Update(w, r)
					return
				}
			}

			memberAPI.Detail(w, r)
		})),
	)

	http.Handle(
		"/api/member/test",
		auth.RequireRole("MEMBER", memberTestHandler),
	)

	http.Handle(
		"/api/member/profile",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberProfileAPI.Get(w, r, user.ID)
		})),
	)

	http.Handle(
		"/api/member/access",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberAccessAPI.Get(w, r, user.ID)
		})),
	)

	http.Handle(
		"/api/member/session",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberSessionAPI.Start(w, r, user.ID)
		})),
	)

	http.Handle(
		"/api/member/sessions/active",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberActiveSessionAPI.Get(w, r, user.ID)
		})),
	)

	http.Handle(
		"/api/member/session/stop",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberSessionStopAPI.Stop(w, r, user.ID)
		})),
	)

	http.Handle(
		"/api/member/usage",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberUsageAPI.Record(w, r, user.ID)
		})),
	)

	http.Handle(
		"/api/member/usage/history",
		auth.RequireRole("MEMBER", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := middleware.CurrentUser(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			memberUsageAPI.History(w, r, user.ID)
		})),
	)

	http.Handle("/api/admin/usage", auth.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminUsageAPI.History(w, r)
	})))
	http.Handle(
		"/api/technician/test",
		auth.RequireRole("TECHNICIAN", technicianTestHandler),
	)

	log.Println("======================================")
	log.Println(" WiFi Management System")
	log.Println(" Backend berjalan di :8080")
	log.Println("======================================")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)

	json.NewEncoder(w).Encode(map[string]string{
		"status": "METHOD_NOT_ALLOWED",
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "UNAUTHORIZED",
		"message": "Session tidak ditemukan",
	})
}
