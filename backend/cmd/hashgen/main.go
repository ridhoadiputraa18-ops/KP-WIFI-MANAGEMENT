package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

func makeHash(password string) string {
	salt := make([]byte, 16)

	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		1,
		64*1024,
		4,
		32,
	)

	return "argon2id$" +
		base64.RawStdEncoding.EncodeToString(salt) +
		"$" +
		base64.RawStdEncoding.EncodeToString(hash)
}

func main() {
	fmt.Println("ADMIN_HASH=" + makeHash("smteladmin123"))
	fmt.Println("TECHNICIAN_HASH=" + makeHash("smtelteknisi124"))
}
