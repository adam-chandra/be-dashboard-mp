package handlers

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	Email        string `json:"email"`
	Role         string `json:"role"`
	AllowedKanal string `json:"allowed_kanal"`
}

func HandleLogin(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Format request login tidak valid"})
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		pass := strings.TrimSpace(req.Password)

		if email == "" || pass == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email dan password wajib diisi"})
		}

		var dbPassword string
		var res UserResponse

		query := "SELECT email, password, role, allowed_kanal FROM users WHERE LOWER(email) = $1 LIMIT 1"
		err := db.QueryRow(query, email).Scan(&res.Email, &dbPassword, &res.Role, &res.AllowedKanal)

		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Email atau password tidak valid"})
		} else if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Terjadi kesalahan pada server"})
		}

		err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(pass))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Email atau password tidak valid"})
		}

		return c.JSON(res)
	}
}
