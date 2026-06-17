package queries

import (
	"Ethos_Sales_Go/be-dashboard-mp/database"
	"Ethos_Sales_Go/be-dashboard-mp/models"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// cacheKey men-derive key deterministik dari payload: "op:sha1(json)".
// SHA-1 cukup untuk identitas cache (bukan kriptografi).
func cacheKey(op string, payload interface{}) string {
	b, _ := json.Marshal(payload)
	sum := sha1.Sum(b)
	return op + ":" + hex.EncodeToString(sum[:])
}

// TTL berbeda per jenis data:
//   - data analitik time-series → CacheTTL() default (60s)
//   - dropdown options & sales performance → 5 menit (jarang berubah)
//   - status logistik → 30s (lebih cepat berubah)
var (
	ttlOptions  = 5 * time.Minute
	ttlSales    = 3 * time.Minute
	ttlLogistik = 30 * time.Second
)

func GetMetrikPerbandinganCached(req models.FilterRequest) (models.MetrikResponse, error) {
	return database.Cached(cacheKey("metrik", req), func() (models.MetrikResponse, error) {
		return GetMetrikPerbandingan(req)
	})
}

func GetTrenGrafikCached(req models.FilterRequest) ([]models.TrenGrafik, error) {
	return database.Cached(cacheKey("grafik", req), func() ([]models.TrenGrafik, error) {
		return GetTrenGrafik(req)
	})
}

func GetPeringkatOutletCached(req models.FilterRequest) ([]map[string]interface{}, error) {
	return database.Cached(cacheKey("peringkat", req), func() ([]map[string]interface{}, error) {
		return GetPeringkatOutlet(req)
	})
}

func GetStatusLogistikCached(req models.FilterRequest) ([]models.StatusLogistik, error) {
	return database.CachedTTL(cacheKey("logistik", req), ttlLogistik, func() ([]models.StatusLogistik, error) {
		return GetStatusLogistik(req)
	})
}

func GetDropdownOptionsCached(channel, kanal, namaToko, kategori, product, brand, team []string) (models.OptionsResponse, error) {
	key := fmt.Sprintf("options:%v|%v|%v|%v|%v|%v|%v", channel, kanal, namaToko, kategori, product, brand, team)
	return database.CachedTTL(key, ttlOptions, func() (models.OptionsResponse, error) {
		return GetDropdownOptions(channel, kanal, namaToko, kategori, product, brand, team)
	})
}

func GetRingkasanHarianCached(req models.FilterRequest) (models.RingkasanResponse, error) {
	return database.Cached(cacheKey("ringkasan", req), func() (models.RingkasanResponse, error) {
		return GetRingkasanHarian(req)
	})
}

func GetSalesPerformanceCached(req models.FilterRequest) (models.SalesPerformanceResponse, error) {
	return database.CachedTTL(cacheKey("sales", req), ttlSales, func() (models.SalesPerformanceResponse, error) {
		return GetSalesPerformance(req)
	})
}
