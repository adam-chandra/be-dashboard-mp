package database

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func SeedUsers(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(150) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		role VARCHAR(50) NOT NULL,
		allowed_kanal VARCHAR(50) NOT NULL
	);`

	if _, err := db.Exec(query); err != nil {
		return err
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	users := []struct {
		Email        string
		Password     string
		Role         string
		AllowedKanal string
	}{
		{"admin@ethos.com", "admin123", "Administrator", "ALL"},
		{"tiktok@ethos.com", "tiktok123", "Tiktok Channel Manager", "Tiktok Shop"},
		{"shopee@ethos.com", "shopee123", "Shopee Channel Manager", "Shopee"},
		{"lazada@ethos.com", "lazada123", "Lazada Channel Manager", "Lazada"},
	}
	for _, u := range users {
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err = db.Exec(
			"INSERT INTO users (email, password, role, allowed_kanal) VALUES ($1, $2, $3, $4)",
			u.Email, string(hashed), u.Role, u.AllowedKanal,
		); err != nil {
			return err
		}
	}
	return nil
}
