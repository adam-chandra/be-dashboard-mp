package handlers

import (
	"Ethos_Sales_Go/be-dashboard-mp/database"

	"github.com/gofiber/fiber/v2"
)

// HandleHealth: liveness + readiness ringan untuk load balancer / probe k8s.
// Mengembalikan status DB & Redis tanpa memicu query mahal.
func HandleHealth(c *fiber.Ctx) error {
	dbOk := false
	if database.DB != nil {
		dbOk = database.DB.Ping() == nil
	}
	status := "ok"
	if !dbOk {
		status = "degraded"
	}
	return c.JSON(fiber.Map{
		"status":        status,
		"db":            dbOk,
		"redis":         database.RedisAvailable(),
		"cache_ttl_sec": int(database.CacheTTL().Seconds()),
	})
}

// HandleInvalidateCache: kosongkan L1 + L2 prefix utama. Berguna setelah
// data warehouse melakukan refresh harian (panggil dari ETL via webhook).
func HandleInvalidateCache(c *fiber.Ctx) error {
	database.InvalidateAll()
	return c.JSON(fiber.Map{"status": "invalidated"})
}
