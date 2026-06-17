package main

import (
	"Ethos_Sales_Go/be-dashboard-mp/database"
	"Ethos_Sales_Go/be-dashboard-mp/handlers"
	"Ethos_Sales_Go/be-dashboard-mp/models"
	"Ethos_Sales_Go/be-dashboard-mp/queries"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func main() {
	if err := godotenv.Load(); err != nil {
		if err = godotenv.Load("../.env"); err != nil {
			log.Println("[ENV] no .env file found")
		}
	}

	database.InitDB(database.LoadDBConfig())
	database.InitRedis(database.LoadRedisConfig())
	if err := database.SeedUsers(database.DB); err != nil {
		log.Printf("[SEED] %v", err)
	}

	app := fiber.New(fiber.Config{
		AppName:               "Executive Analytics API v2.1 (mp)",
		ReadBufferSize:        16384,
		DisableStartupMessage: false,
		// Concurrency default fiber 256K cukup; biarkan default.
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	app.Use(etag.New()) // ETag → 304 Not Modified untuk payload identik

	// CORS: Fiber panic bila AllowOrigins="*" dipasangkan dengan AllowCredentials=true,
	// jadi auto-nonaktifkan credentials saat wildcard digunakan.
	corsOrigins := getEnv("CORS_ORIGINS", "http://localhost:5173")
	allowCreds := corsOrigins != "*"
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, If-None-Match",
		AllowMethods:     "GET, POST, HEAD, PUT, DELETE, OPTIONS",
		AllowCredentials: allowCreds,
		ExposeHeaders:    "ETag",
	}))

	// ---------- Health & cache ops ----------
	app.Get("/api/health", handlers.HandleHealth)
	app.Post("/api/cache/invalidate", handlers.HandleInvalidateCache)

	// ---------- Analitik ----------
	app.Post("/api/metrik", func(c *fiber.Ctx) error {
		var req models.FilterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid metrik payload"})
		}
		req.Normalize()
		data, err := queries.GetMetrikPerbandinganCached(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Post("/api/grafik", func(c *fiber.Ctx) error {
		var req models.FilterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid grafik payload"})
		}
		req.Normalize()
		data, err := queries.GetTrenGrafikCached(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Post("/api/peringkat", func(c *fiber.Ctx) error {
		var req models.FilterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid peringkat payload"})
		}
		req.Normalize()
		data, err := queries.GetPeringkatOutletCached(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Post("/api/logistik", func(c *fiber.Ctx) error {
		var req models.FilterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid logistik payload"})
		}
		req.Normalize()
		data, err := queries.GetStatusLogistikCached(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Get("/api/options", func(c *fiber.Ctx) error {
		readMulti := func(name string) []string {
			out := []string{}
			for _, b := range c.Context().QueryArgs().PeekMulti(name) {
				s := string(b)
				if s == "" {
					continue
				}
				out = append(out, splitCSV(s)...)
			}
			return out
		}
		channel := models.NormalizeOptionParam(readMulti("channel"))
		kanal := models.NormalizeOptionParam(readMulti("kanal"))
		namaToko := models.NormalizeOptionParam(readMulti("nama_toko"))
		kategori := models.NormalizeOptionParam(readMulti("kategori"))
		productName := models.NormalizeOptionParam(readMulti("product_name"))
		brand := models.NormalizeOptionParam(readMulti("brand"))
		team := models.NormalizeOptionParam(readMulti("team"))

		data, err := queries.GetDropdownOptionsCached(channel, kanal, namaToko, kategori, productName, brand, team)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Post("/api/ringkasan", func(c *fiber.Ctx) error {
		var req models.FilterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid ringkasan payload"})
		}
		req.Normalize()
		data, err := queries.GetRingkasanHarianCached(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Post("/api/sales-performance", func(c *fiber.Ctx) error {
		var req models.FilterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid sales-performance payload"})
		}
		req.Normalize()
		data, err := queries.GetSalesPerformanceCached(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	app.Post("/api/login", handlers.HandleLogin(database.DB))

	// Host binding: Render / Fly / Cloud Run mengisi env PORT secara dinamis;
	// fallback ke APP_PORT (dev lokal) lalu 8084.
	port := getEnv("PORT", getEnv("APP_PORT", "8084"))
	log.Fatal(app.Listen("0.0.0.0:" + port))
}
