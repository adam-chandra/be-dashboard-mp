package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
)

var DB *sql.DB

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	Schema   string
}

func LoadDBConfig() *DBConfig {
	return &DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", ""),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:   getEnv("DB_NAME", ""),
		Schema:   getEnv("DB_SCHEMA", "public"),
	}
}

func InitDB(cfg *DBConfig) {
	// DATABASE_URL (postgres://user:pass@host:port/db?sslmode=require) lebih
	// diutamakan agar gampang dipakai di Render/Fly/Railway.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		sslmode := getEnv("DB_SSLMODE", "disable")
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s search_path=%s sslmode=%s statement_cache_mode=describe",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.Schema, sslmode,
		)
	}

	var err error
	DB, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("[DB] open failed: %v", err)
	}

	DB.SetMaxOpenConns(50)
	DB.SetMaxIdleConns(15)
	DB.SetConnMaxLifetime(10 * time.Minute)
	DB.SetConnMaxIdleTime(5 * time.Minute)

	if err = DB.Ping(); err != nil {
		log.Fatalf("[DB] ping failed: %v", err)
	}
	fmt.Println("[DB] connected to PostgreSQL")

	ensureIndexes()
}

// ensureIndexes membuat indeks komposit + ekspresi LOWER() yang sering dipakai
// oleh seluruh query analitik. Idempoten (IF NOT EXISTS). Ada beberapa indeks
// tambahan dibanding versi lama untuk men-cover pattern query baru:
//   - INCLUDE (omzet, quantity) → index-only scan untuk agregasi
//   - BRIN di date → murah & efektif untuk tabel append-only besar
//   - composite LOWER(brand, kategori) untuk leaderboard & hero products
func ensureIndexes() {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_marts_date_brin
		   ON marts_ecommerce USING BRIN (date) WITH (pages_per_range = 32);`,

		`CREATE INDEX IF NOT EXISTS idx_marts_date_btree
		   ON marts_ecommerce (date);`,

		`CREATE INDEX IF NOT EXISTS idx_marts_kanal_lower
		   ON marts_ecommerce (LOWER(kanal));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_channel_lower
		   ON marts_ecommerce (LOWER(channel));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_toko_lower
		   ON marts_ecommerce (LOWER(toko));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_brand_lower
		   ON marts_ecommerce (LOWER(brand));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_kategori_lower
		   ON marts_ecommerce (LOWER(kategori));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_product_name_lower
		   ON marts_ecommerce (LOWER(product_name));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_team_lower
		   ON marts_ecommerce (LOWER(team));`,

		`CREATE INDEX IF NOT EXISTS idx_marts_date_kanal_brand
		   ON marts_ecommerce (date, LOWER(kanal), LOWER(brand));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_date_channel_toko
		   ON marts_ecommerce (date, LOWER(channel), LOWER(toko));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_date_kategori_product
		   ON marts_ecommerce (date, LOWER(kategori), LOWER(product_name));`,
		`CREATE INDEX IF NOT EXISTS idx_marts_date_team
		   ON marts_ecommerce (date, LOWER(team));`,

		`CREATE INDEX IF NOT EXISTS idx_marts_date_brand_toko
		   ON marts_ecommerce (date, LOWER(brand), LOWER(toko))
		   INCLUDE (omzet, quantity, product_name, product_code_new);`,

		`CREATE INDEX IF NOT EXISTS idx_marts_product_name_upper
		   ON marts_ecommerce (UPPER(product_name));`,

		`CREATE INDEX IF NOT EXISTS idx_marts_order_number
		   ON marts_ecommerce (order_number);`,
		`CREATE INDEX IF NOT EXISTS idx_marts_product_code_new
		   ON marts_ecommerce (product_code_new);`,

		`CREATE INDEX IF NOT EXISTS idx_marts_date_kanal_brand_kategori
		   ON marts_ecommerce (date, LOWER(kanal), LOWER(brand), LOWER(kategori))
		   INCLUDE (omzet, quantity, order_number, product_code_new);`,

		`CREATE INDEX IF NOT EXISTS idx_marts_date_include_agg
		   ON marts_ecommerce (date)
		   INCLUDE (omzet, quantity, product_code_new, order_number);`,

		`CREATE INDEX IF NOT EXISTS idx_marts_brand_kategori_lower
		   ON marts_ecommerce (LOWER(brand), LOWER(kategori));`,
	}
	for _, stmt := range statements {
		if _, err := DB.Exec(stmt); err != nil {
			log.Printf("[DB] index skipped: %v", err)
		}
	}
	if _, err := DB.Exec(`ANALYZE marts_ecommerce;`); err != nil {
		log.Printf("[DB] analyze skipped: %v", err)
	}
	fmt.Println("[DB] composite + lower() indexes ensured")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
