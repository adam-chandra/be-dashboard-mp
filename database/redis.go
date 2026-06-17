package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient adalah handle global ke server Redis L2 cache.
// Bila nil atau redisAlive==0, semua operasi Redis di-skip (graceful fallback).
var (
	RedisClient *redis.Client
	redisAlive  int32 // 0 = off, 1 = on; di-update oleh InitRedis dan health check
	redisCtx    = context.Background()
)

type RedisConfig struct {
	URL      string
	Addr     string
	Password string
	DB       int
	Enabled  bool
}

// LoadRedisConfig membaca konfigurasi Redis dari environment.
// Jika REDIS_ADDR & REDIS_URL kosong → Redis dianggap dimatikan (mode in-memory only).
// REDIS_URL (rediss://...) lebih diutamakan agar kompatibel dengan Upstash.
func LoadRedisConfig() RedisConfig {
	url := os.Getenv("REDIS_URL")
	addr := os.Getenv("REDIS_ADDR")
	enabled := url != "" || addr != ""
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			db = n
		}
	}
	return RedisConfig{
		URL:      url,
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
		Enabled:  enabled,
	}
}

// InitRedis mencoba koneksi ke Redis. Kegagalan tidak menghentikan aplikasi —
// cukup log dan biarkan layer L1 (in-memory SimpleCache) menangani semuanya.
func InitRedis(cfg RedisConfig) {
	if !cfg.Enabled {
		log.Println("[REDIS] disabled (REDIS_ADDR/REDIS_URL empty) — in-memory cache only")
		return
	}

	var client *redis.Client
	if cfg.URL != "" {
		opts, err := redis.ParseURL(cfg.URL)
		if err != nil {
			log.Printf("[REDIS] invalid REDIS_URL (%v) — fallback to in-memory cache", err)
			return
		}
		opts.DialTimeout = 2 * time.Second
		opts.ReadTimeout = 500 * time.Millisecond
		opts.WriteTimeout = 500 * time.Millisecond
		opts.PoolSize = 20
		opts.MinIdleConns = 4
		client = redis.NewClient(opts)
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:         cfg.Addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
			PoolSize:     20,
			MinIdleConns: 4,
		})
	}

	pingCtx, cancel := context.WithTimeout(redisCtx, 2*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		log.Printf("[REDIS] ping failed (%v) — fallback to in-memory cache", err)
		_ = client.Close()
		return
	}

	RedisClient = client
	atomic.StoreInt32(&redisAlive, 1)
	if cfg.URL != "" {
		fmt.Println("[REDIS] connected via REDIS_URL")
	} else {
		fmt.Printf("[REDIS] connected to %s db=%d\n", cfg.Addr, cfg.DB)
	}

	go redisHealthLoop()
}

// redisHealthLoop ping Redis tiap 30s; mark up/down agar Cached() bisa
// fallback ke L1 tanpa hang menunggu network timeout per request.
func redisHealthLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		if RedisClient == nil {
			return
		}
		ctx, cancel := context.WithTimeout(redisCtx, 1*time.Second)
		err := RedisClient.Ping(ctx).Err()
		cancel()
		if err != nil {
			if atomic.SwapInt32(&redisAlive, 0) == 1 {
				log.Printf("[REDIS] unhealthy: %v — switching to in-memory only", err)
			}
			continue
		}
		if atomic.SwapInt32(&redisAlive, 1) == 0 {
			log.Println("[REDIS] back online — L2 cache re-enabled")
		}
	}
}

func RedisAvailable() bool {
	return RedisClient != nil && atomic.LoadInt32(&redisAlive) == 1
}

func RedisGetJSON(key string, out interface{}) (bool, error) {
	if !RedisAvailable() {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(redisCtx, 500*time.Millisecond)
	defer cancel()
	b, err := RedisClient.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return true, json.Unmarshal(b, out)
}

func RedisSetJSON(key string, val interface{}, ttl time.Duration) {
	if !RedisAvailable() {
		return
	}
	b, err := json.Marshal(val)
	if err != nil {
		log.Printf("[REDIS] marshal error key=%s: %v", key, err)
		return
	}
	ctx, cancel := context.WithTimeout(redisCtx, 500*time.Millisecond)
	defer cancel()
	if err := RedisClient.Set(ctx, key, b, ttl).Err(); err != nil {
		log.Printf("[REDIS] set error key=%s: %v", key, err)
	}
}

// RedisInvalidatePrefix menghapus semua key dengan prefix tertentu (mis. "metrik:*")
// memakai SCAN agar tidak blocking server Redis.
func RedisInvalidatePrefix(prefix string) {
	if !RedisAvailable() {
		return
	}
	ctx, cancel := context.WithTimeout(redisCtx, 5*time.Second)
	defer cancel()
	iter := RedisClient.Scan(ctx, 0, prefix+"*", 200).Iterator()
	batch := make([]string, 0, 200)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		_ = RedisClient.Del(ctx, batch...).Err()
		batch = batch[:0]
	}
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) >= 200 {
			flush()
		}
	}
	flush()
}
