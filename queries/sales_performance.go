package queries

import (
	"Ethos_Sales_Go/be-dashboard-mp/database"
	"Ethos_Sales_Go/be-dashboard-mp/models"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var nplKodeList = []string{
	"BKL-R01", "CKM-R01", "ETH-R01", "GTF-R01", "LIN-S04", "ETF-R01",
}

func pickGeoColumn(candidates ...string) string {
	cols := getMartsColumns()
	for _, c := range candidates {
		if cols[strings.ToLower(c)] {
			return fmt.Sprintf("COALESCE(NULLIF(TRIM(%s::text), ''), '')", c)
		}
	}
	return ""
}

const topContribN = 7

func queryContribution(req models.FilterRequest, dim string) ([]models.ContribItem, error) {
	whr, args := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 3)

	queryArgs := make([]interface{}, 0, 2+len(args))
	queryArgs = append(queryArgs, req.TanggalMulai, req.TanggalSelesai)
	queryArgs = append(queryArgs, args...)

	q := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(TRIM(%s), ''), 'Lainnya') AS label,
			COALESCE(SUM(omzet), 0)::float8         AS total_gmv,
			COALESCE(SUM(quantity), 0)::bigint      AS qty
		FROM marts_ecommerce
		WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE) %s
		GROUP BY label
		ORDER BY total_gmv DESC
	`, dim, whr)

	rows, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	all := make([]models.ContribItem, 0, 64)
	for rows.Next() {
		var it models.ContribItem
		if err := rows.Scan(&it.Label, &it.Omset, &it.Qty); err == nil {
			all = append(all, it)
		}
	}

	if len(all) <= topContribN {
		return all, nil
	}
	out := make([]models.ContribItem, 0, topContribN+1)
	out = append(out, all[:topContribN]...)

	var othersGmv float64
	var othersQty int64
	for _, it := range all[topContribN:] {
		othersGmv += it.Omset
		othersQty += it.Qty
	}
	if othersGmv > 0 {
		merged := false
		for i := range out {
			if strings.EqualFold(strings.TrimSpace(out[i].Label), "Lainnya") {
				out[i].Omset += othersGmv
				out[i].Qty += othersQty
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, models.ContribItem{Label: "Lainnya", Omset: othersGmv, Qty: othersQty})
		}
	}
	return out, nil
}

func queryGeographic(req models.FilterRequest, level string) ([]models.GeoItem, error) {
	whr, args := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 3)

	var col string
	switch level {
	case "kota":
		col = pickGeoColumn("kota", "city")
	case "provinsi":
		col = pickGeoColumn("provinsi", "province", "propinsi")
	}
	if col == "" {
		col = `COALESCE(NULLIF(TRIM(SPLIT_PART(toko, ' ', GREATEST(1, ARRAY_LENGTH(STRING_TO_ARRAY(toko, ' '), 1)))), ''), 'Lainnya')`
	}

	limit := 500
	var queryArgs []interface{}
	queryArgs = append(queryArgs, req.TanggalMulai, req.TanggalSelesai)
	queryArgs = append(queryArgs, args...)

	q := fmt.Sprintf(`
		SELECT
			%s AS wilayah,
			COALESCE(SUM(omzet), 0)::float8 AS omset
		FROM marts_ecommerce
		WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE) %s
		GROUP BY wilayah
		HAVING COALESCE(SUM(omzet), 0) > 0
		ORDER BY omset DESC
		LIMIT %d
	`, col, whr, limit)

	rows, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.GeoItem{}
	for rows.Next() {
		var it models.GeoItem
		if err := rows.Scan(&it.Wilayah, &it.Omset); err == nil {
			if strings.TrimSpace(it.Wilayah) == "" {
				it.Wilayah = "Lainnya"
			}
			out = append(out, it)
		}
	}
	return out, nil
}

func queryGeographicProvinsi(req models.FilterRequest) ([]models.GeoItem, models.GeoMappingStats, error) {
	whr, args := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 3)

	provCol := pickGeoColumn("provinsi", "province", "propinsi")
	if provCol == "" {
		provCol = "''::text"
	}
	kotaCol := pickGeoColumn("kota", "city")
	if kotaCol == "" {
		kotaCol = "''::text"
	}

	var queryArgs []interface{}
	queryArgs = append(queryArgs, req.TanggalMulai, req.TanggalSelesai)
	queryArgs = append(queryArgs, args...)

	q := fmt.Sprintf(`
		SELECT
			%s AS raw_provinsi,
			%s AS raw_kota,
			COALESCE(SUM(omzet), 0)::float8 AS omset
		FROM marts_ecommerce
		WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE) %s
		GROUP BY raw_provinsi, raw_kota
		HAVING COALESCE(SUM(omzet), 0) > 0
	`, provCol, kotaCol, whr)

	rows, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, models.GeoMappingStats{}, err
	}
	defer rows.Close()

	agg := map[string]float64{}
	var mapped, unmapped int64

	for rows.Next() {
		var rawProv, rawKota string
		var omset float64
		if err := rows.Scan(&rawProv, &rawKota, &omset); err != nil {
			continue
		}
		canonical := resolveProvinsi(rawProv, rawKota)
		if canonical == "" {
			unmapped++
			continue
		}
		mapped++
		agg[canonical] += omset
	}

	out := make([]models.GeoItem, 0, len(agg))
	for k, v := range agg {
		out = append(out, models.GeoItem{Wilayah: k, Omset: v})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Omset > out[j].Omset })
	if len(out) > 500 {
		out = out[:500]
	}
	return out, models.GeoMappingStats{MappedCount: mapped, UnmappedCount: unmapped}, nil
}

func queryHeroProducts(req models.FilterRequest) ([]models.HeroProduct, float64, error) {
	whr, args := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 3)

	var queryArgs []interface{}
	queryArgs = append(queryArgs, req.TanggalMulai, req.TanggalSelesai)
	queryArgs = append(queryArgs, args...)

	q := fmt.Sprintf(`
		WITH base AS (
			SELECT 
				COALESCE(NULLIF(TRIM(product_name), ''), 'Unknown Produk') AS produk,
				COALESCE(NULLIF(TRIM(brand), ''), 'Unbranded')             AS brand,
				COALESCE(NULLIF(TRIM(kategori), ''), 'Lainnya')            AS kategori,
				COALESCE(omzet, 0)::float8 AS omzet
			FROM marts_ecommerce
			WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE) %s
		), agg AS (
			SELECT produk, MAX(brand) AS brand, MAX(kategori) AS kategori, SUM(omzet) AS gmv
			FROM base
			GROUP BY produk
		), tot AS (
			SELECT COALESCE(SUM(gmv), 0)::float8 AS total FROM agg
		)
		SELECT a.produk, a.brand, a.kategori, a.gmv,
			CASE WHEN t.total = 0 THEN 0 ELSE (a.gmv / t.total) END AS share,
			t.total
		FROM agg a CROSS JOIN tot t
		WHERE t.total > 0 AND (a.gmv / t.total) > 0.20
		ORDER BY share DESC
		LIMIT 20
	`, whr)

	rows, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []models.HeroProduct{}
	var total float64
	for rows.Next() {
		var h models.HeroProduct
		var t float64
		if err := rows.Scan(&h.ProductName, &h.Brand, &h.Kategori, &h.GMV, &h.Share, &t); err == nil {
			out = append(out, h)
			total = t
		}
	}
	if total == 0 {
		qTot := fmt.Sprintf(`
			SELECT COALESCE(SUM(omzet), 0)::float8 FROM marts_ecommerce
			WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE) %s
		`, whr)
		_ = database.DB.QueryRow(qTot, queryArgs...).Scan(&total)
	}
	return out, total, nil
}

// queryNPLSeries OPTIMIZED: dulu loop 6× query (N+1 round-trip).
// Sekarang satu query gabungan via VALUES(kode, pattern) sehingga DB-nya
// scan tabel sekali dan join LATERAL untuk hitung d30/d60/d90 tiap kode.
func queryNPLSeries(req models.FilterRequest) ([]models.NPLSeries, error) {
	whr, args := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 1+2*len(nplKodeList))

	// VALUES(kode, pattern) → $1=kode1, $2=pattern1, $3=kode2, $4=pattern2, ...
	queryArgs := make([]interface{}, 0, 2*len(nplKodeList)+len(args))
	rowsValues := make([]string, 0, len(nplKodeList))
	for i, kode := range nplKodeList {
		queryArgs = append(queryArgs, kode, "%"+strings.ToUpper(kode)+"%")
		rowsValues = append(rowsValues, fmt.Sprintf("($%d, $%d)", 2*i+1, 2*i+2))
	}
	queryArgs = append(queryArgs, args...)

	q := fmt.Sprintf(`
		WITH codes(kode, pattern) AS (VALUES %s),
		matched AS (
			SELECT c.kode,
			       m.date,
			       COALESCE(m.omzet, 0)::float8 AS omzet,
			       COALESCE(m.quantity, 0)::bigint AS qty,
			       COALESCE(NULLIF(TRIM(m.product_name), ''), '') AS pname
			FROM codes c
			JOIN marts_ecommerce m ON UPPER(m.product_name) LIKE c.pattern %s
		),
		launch AS (
			SELECT kode, MIN(date) AS d0 FROM matched GROUP BY kode
		)
		SELECT 
			c.kode,
			COALESCE(MAX(m.pname), '') AS pname,
			COALESCE(SUM(CASE WHEN m.date <= (l.d0 + INTERVAL '30 days') THEN m.omzet END), 0)::float8 AS s30,
			COALESCE(SUM(CASE WHEN m.date <= (l.d0 + INTERVAL '60 days') THEN m.omzet END), 0)::float8 AS s60,
			COALESCE(SUM(CASE WHEN m.date <= (l.d0 + INTERVAL '90 days') THEN m.omzet END), 0)::float8 AS s90,
			COALESCE(SUM(CASE WHEN m.date <= (l.d0 + INTERVAL '30 days') THEN m.qty   END), 0)::bigint AS q30,
			COALESCE(SUM(CASE WHEN m.date <= (l.d0 + INTERVAL '60 days') THEN m.qty   END), 0)::bigint AS q60,
			COALESCE(SUM(CASE WHEN m.date <= (l.d0 + INTERVAL '90 days') THEN m.qty   END), 0)::bigint AS q90,
			COUNT(m.*)::bigint AS sampel
		FROM codes c
		LEFT JOIN matched m ON m.kode = c.kode
		LEFT JOIN launch  l ON l.kode = c.kode
		GROUP BY c.kode
	`, strings.Join(rowsValues, ", "), whr)

	rs, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	byKode := make(map[string]models.NPLSeries, len(nplKodeList))
	for rs.Next() {
		var s models.NPLSeries
		if err := rs.Scan(&s.Kode, &s.ProductName, &s.Day30, &s.Day60, &s.Day90, &s.QtyD30, &s.QtyD60, &s.QtyD90, &s.Sampel); err == nil {
			s.Found = s.Sampel > 0
			byKode[s.Kode] = s
		}
	}

	out := make([]models.NPLSeries, 0, len(nplKodeList))
	for _, k := range nplKodeList {
		if s, ok := byKode[k]; ok {
			out = append(out, s)
		} else {
			out = append(out, models.NPLSeries{Kode: k, Found: false})
		}
	}
	return out, nil
}

// discoverRecentLaunches OPTIMIZED: dulu N+1 (1 query discover + N query per produk).
// Sekarang gabung jadi 1 query: ambil top-N nama produk dengan launch_date terbaru,
// lalu satu pass agregasi window d30/60/90 dengan JOIN pada produk-produk tsb.
func discoverRecentLaunches(req models.FilterRequest, limit int) ([]models.NPLSeries, error) {
	whrM, argsM := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 2)

	queryArgs := make([]interface{}, 0, 1+len(argsM))
	queryArgs = append(queryArgs, limit)
	queryArgs = append(queryArgs, argsM...)

	q := fmt.Sprintf(`
		WITH base AS (
			SELECT
				COALESCE(NULLIF(TRIM(product_name), ''), '') AS pname,
				date,
				COALESCE(omzet, 0)::float8 AS omzet,
				COALESCE(quantity, 0)::bigint AS qty
			FROM marts_ecommerce
			WHERE COALESCE(NULLIF(TRIM(product_name), ''), '') <> '' %s
		),
		recent AS (
			SELECT pname, MIN(date) AS d0, COUNT(*) AS sampel
			FROM base
			GROUP BY pname
			ORDER BY d0 DESC NULLS LAST, sampel DESC
			LIMIT $1
		)
		SELECT
			r.pname,
			COALESCE(SUM(CASE WHEN b.date <= (r.d0 + INTERVAL '30 days') THEN b.omzet END), 0)::float8 AS s30,
			COALESCE(SUM(CASE WHEN b.date <= (r.d0 + INTERVAL '60 days') THEN b.omzet END), 0)::float8 AS s60,
			COALESCE(SUM(CASE WHEN b.date <= (r.d0 + INTERVAL '90 days') THEN b.omzet END), 0)::float8 AS s90,
			COALESCE(SUM(CASE WHEN b.date <= (r.d0 + INTERVAL '30 days') THEN b.qty   END), 0)::bigint AS q30,
			COALESCE(SUM(CASE WHEN b.date <= (r.d0 + INTERVAL '60 days') THEN b.qty   END), 0)::bigint AS q60,
			COALESCE(SUM(CASE WHEN b.date <= (r.d0 + INTERVAL '90 days') THEN b.qty   END), 0)::bigint AS q90,
			COUNT(*)::bigint AS sampel
		FROM recent r
		JOIN base b ON b.pname = r.pname
		GROUP BY r.pname, r.d0
		ORDER BY r.d0 DESC NULLS LAST, sampel DESC
	`, whrM)

	rows, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("discover query: %w", err)
	}
	defer rows.Close()

	out := make([]models.NPLSeries, 0, limit)
	for rows.Next() {
		var pname string
		var s30, s60, s90 float64
		var q30, q60, q90, sampel int64
		if err := rows.Scan(&pname, &s30, &s60, &s90, &q30, &q60, &q90, &sampel); err != nil {
			continue
		}
		kode := strings.ToUpper(pname)
		if len(kode) > 12 {
			kode = kode[:12] + "…"
		}
		out = append(out, models.NPLSeries{
			Kode:           kode,
			ProductName:    pname,
			Day30:          s30,
			Day60:          s60,
			Day90:          s90,
			QtyD30:         q30,
			QtyD60:         q60,
			QtyD90:         q90,
			Sampel:         sampel,
			Found:          sampel > 0,
			AutoDiscovered: true,
		})
	}
	return out, nil
}

func queryLeaderboard(req models.FilterRequest) ([]models.LeaderboardItem, error) {
	whr, filterArgs := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)

	currStart, currEnd, prevStart, prevEnd, err := getPeriodRanges(req.TanggalMulai, req.TanggalSelesai)
	if err != nil {
		return nil, err
	}

	var queryArgs []interface{}
	queryArgs = append(queryArgs, currStart, currEnd, prevStart, prevEnd)
	queryArgs = append(queryArgs, filterArgs...)

	// Leaderboard: dijadikan SINGLE scan dengan FILTER (current vs previous).
	q := fmt.Sprintf(`
		WITH agg AS (
			SELECT
				COALESCE(NULLIF(TRIM(product_name), ''), 'Unknown Produk') AS produk,
				MAX(COALESCE(NULLIF(TRIM(brand), ''), 'Unbranded'))         AS brand,
				COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)::float8 AS gmv,
				COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0)::float8 AS gmv_prev,
				COALESCE(SUM(quantity) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)::bigint AS qty,
				COUNT(DISTINCT NULLIF(TRIM(order_number), '')) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE))::bigint AS orders
			FROM marts_ecommerce
			WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
			    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) %s
			GROUP BY produk
		)
		SELECT produk, brand, gmv, gmv_prev, qty, orders
		FROM agg
		WHERE gmv > 0
		ORDER BY gmv DESC
		LIMIT 30
	`, whr)

	rows, err := database.DB.Query(q, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.LeaderboardItem{}
	for rows.Next() {
		var it models.LeaderboardItem
		var qty, orders int64
		if err := rows.Scan(&it.ProductName, &it.Brand, &it.GMV, &it.GMVLalu, &qty, &orders); err == nil {
			if it.GMVLalu > 0 {
				it.Growth = ((it.GMV - it.GMVLalu) / it.GMVLalu) * 100.0
			}
			if orders > 0 {
				it.AOV = it.GMV / float64(orders)
			}
			if qty > 0 {
				it.ASP = it.GMV / float64(qty)
			}
			out = append(out, it)
		}
	}
	for i := range out {
		out[i].Peringkat = i + 1
	}
	return out, nil
}

func GetSalesPerformance(req models.FilterRequest) (models.SalesPerformanceResponse, error) {
	var res models.SalesPerformanceResponse

	currStart, currEnd, prevStart, prevEnd, err := getPeriodRanges(req.TanggalMulai, req.TanggalSelesai)
	if err != nil {
		return res, err
	}
	res.PeriodeSekarangMulai = currStart
	res.PeriodeSekarangSelesai = currEnd
	res.PeriodeLaluMulai = prevStart
	res.PeriodeLaluSelesai = prevEnd

	var (
		wg sync.WaitGroup
		mu sync.Mutex

		contribBrand    []models.ContribItem
		contribKanal    []models.ContribItem
		contribKategori []models.ContribItem
		topProvinsi     []models.GeoItem
		topKota         []models.GeoItem
		provMapStats    models.GeoMappingStats
		heroProducts    []models.HeroProduct
		totalOmset      float64
		nplSeries       []models.NPLSeries
		leaderboard     []models.LeaderboardItem
	)

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = fn()
		}()
	}

	run(func() error {
		v, err := queryContribution(req, "brand")
		if err == nil {
			mu.Lock()
			contribBrand = v
			mu.Unlock()
		}
		return err
	})
	run(func() error {
		v, err := queryContribution(req, "kanal")
		if err == nil {
			mu.Lock()
			contribKanal = v
			mu.Unlock()
		}
		return err
	})
	run(func() error {
		v, err := queryContribution(req, "kategori")
		if err == nil {
			mu.Lock()
			contribKategori = v
			mu.Unlock()
		}
		return err
	})
	run(func() error {
		v, stats, err := queryGeographicProvinsi(req)
		if err == nil {
			mu.Lock()
			topProvinsi = v
			provMapStats = stats
			mu.Unlock()
		}
		return err
	})
	run(func() error {
		v, err := queryGeographic(req, "kota")
		if err == nil {
			mu.Lock()
			topKota = v
			mu.Unlock()
		}
		return err
	})
	run(func() error {
		hp, total, err := queryHeroProducts(req)
		if err == nil {
			mu.Lock()
			heroProducts = hp
			totalOmset = total
			mu.Unlock()
		}
		return err
	})
	run(func() error {
		npl, err := queryNPLSeries(req)
		if err != nil {
			return err
		}
		foundCount := 0
		for _, s := range npl {
			if s.Found {
				foundCount++
			}
		}
		if foundCount == 0 {
			if autoNpl, derr := discoverRecentLaunches(req, 6); derr == nil && len(autoNpl) > 0 {
				npl = autoNpl
			} else {
				bareReq := models.FilterRequest{
					TanggalMulai:   req.TanggalMulai,
					TanggalSelesai: req.TanggalSelesai,
				}
				if autoNpl, derr := discoverRecentLaunches(bareReq, 6); derr == nil && len(autoNpl) > 0 {
					npl = autoNpl
				}
			}
		}
		sort.SliceStable(npl, func(i, j int) bool { return npl[i].Sampel > npl[j].Sampel })
		mu.Lock()
		nplSeries = npl
		mu.Unlock()
		return nil
	})
	run(func() error {
		v, err := queryLeaderboard(req)
		if err == nil {
			mu.Lock()
			leaderboard = v
			mu.Unlock()
		}
		return err
	})

	wg.Wait()

	res.ContribByBrand = contribBrand
	res.ContribByKanal = contribKanal
	res.ContribByKategori = contribKategori
	res.TopProvinsi = topProvinsi
	res.ProvinsiMappingStats = provMapStats
	res.TopKota = topKota
	res.HeroProducts = heroProducts
	res.TotalOmset = totalOmset
	res.NPLSeries = nplSeries
	res.Leaderboard = leaderboard

	return res, nil
}
