package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"kp-wifi-management/database"
	"kp-wifi-management/handlers"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	db := database.Open()
	defer db.Close()

	database.Init(db)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Nama lengkap: ")
	fullName, _ := reader.ReadString('\n')
	fullName = strings.TrimSpace(fullName)

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if username == "" || fullName == "" || password == "" {
		fmt.Println("Username, nama lengkap, dan password wajib diisi.")
		return
	}

	var roleID int

	err := db.QueryRow(
		`SELECT id FROM roles WHERE name = 'ADMIN'`,
	).Scan(&roleID)

	if err != nil {
		fmt.Println("Role ADMIN tidak ditemukan:", err)
		return
	}

	passwordHash, err := handlers.GeneratePassword(password)
	if err != nil {
		fmt.Println("Gagal membuat password hash:", err)
		return
	}

	_, err = db.Exec(`
		INSERT INTO users
			(role_id, username, password_hash, full_name, email, status)
		VALUES (?, ?, ?, ?, ?, 'ACTIVE')
	`, roleID, username, passwordHash, fullName, nullString(email))

	if err != nil {
		fmt.Println("Gagal membuat user:", err)
		return
	}

	fmt.Println()
	fmt.Println("======================================")
	fmt.Println(" ADMIN BERHASIL DIBUAT")
	fmt.Println(" Username :", username)
	fmt.Println(" Nama     :", fullName)
	fmt.Println("======================================")
}

func nullString(value string) interface{} {
	if value == "" {
		return sql.NullString{}
	}

	return value
}
