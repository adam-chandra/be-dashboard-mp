package queries

import (
	"Ethos_Sales_Go/be-dashboard-mp/database"
	"Ethos_Sales_Go/be-dashboard-mp/models"
	"fmt"
	"strings"
	"sync"
	"time"
)

func wibLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.UTC
	}
	return loc
}

func getPreviousMonthRange(currentStart string) (string, string, error) {
	layout := "2006-01-02"
	loc := wibLoc()
	dtMulai, err := time.ParseInLocation(layout, currentStart, loc)
	if err != nil {
		return "", "", err
	}
	prevMonthStart := time.Date(dtMulai.Year(), dtMulai.Month()-1, 1, 0, 0, 0, 0, loc)
	prevMonthEnd := time.Date(dtMulai.Year(), dtMulai.Month(), 0, 0, 0, 0, 0, loc)
	return prevMonthStart.Format(layout), prevMonthEnd.Format(layout), nil
}

func shiftToPrevMonth(dateStr string, loc *time.Location) (string, error) {
	layout := "2006-01-02"
	dt, err := time.ParseInLocation(layout, dateStr, loc)
	if err != nil {
		return "", err
	}
	lastDayPrev := time.Date(dt.Year(), dt.Month(), 0, 0, 0, 0, 0, loc).Day()
	day := dt.Day()
	if day > lastDayPrev {
		day = lastDayPrev
	}
	result := time.Date(dt.Year(), dt.Month()-1, day, 0, 0, 0, 0, loc)
	return result.Format(layout), nil
}

func getPeriodRanges(tanggalMulai, tanggalSelesai string) (currStart, currEnd, prevStart, prevEnd string, err error) {
	loc := wibLoc()
	currStart = tanggalMulai
	currEnd = tanggalSelesai
	prevStart, err = shiftToPrevMonth(tanggalMulai, loc)
	if err != nil {
		return
	}
	prevEnd, err = shiftToPrevMonth(tanggalSelesai, loc)
	return
}

func isSemuaOrEmpty(v string) bool {
	t := strings.TrimSpace(v)
	if t == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(t), "semua")
}

func cleanMulti(vals []string) []string {
	if len(vals) == 0 {
		return nil
	}
	out := make([]string, 0, len(vals))
	seen := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		t := strings.TrimSpace(v)
		if t == "" || isSemuaOrEmpty(t) {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

func buildFilterClausesExt(channel, kanal, namaToko, kategori, productName, brand, team []string, startCounter int) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	counter := startCounter

	if vals := cleanMulti(channel); len(vals) > 0 {
		var parts []string
		for _, v := range vals {
			l := strings.ToLower(v)
			parts = append(parts, fmt.Sprintf("LOWER(channel) LIKE $%d", counter))
			switch {
			case strings.Contains(l, "online"):
				args = append(args, "%online%")
			case strings.Contains(l, "offline"):
				args = append(args, "%offline%")
			default:
				args = append(args, "%"+l+"%")
			}
			counter++
		}
		conditions = append(conditions, "("+strings.Join(parts, " OR ")+")")
	}

	addIn := func(col string, vals []string) {
		cleaned := cleanMulti(vals)
		if len(cleaned) == 0 {
			return
		}
		placeholders := make([]string, 0, len(cleaned))
		for _, v := range cleaned {
			placeholders = append(placeholders, fmt.Sprintf("$%d", counter))
			args = append(args, strings.ToLower(v))
			counter++
		}
		conditions = append(conditions, fmt.Sprintf("LOWER(%s) IN (%s)", col, strings.Join(placeholders, ", ")))
	}

	addIn("kanal", kanal)
	addIn("toko", namaToko)
	addIn("kategori", kategori)
	addIn("product_name", productName)
	addIn("brand", brand)
	addIn("team", team)

	if len(conditions) == 0 {
		return "", args
	}
	return " AND " + strings.Join(conditions, " AND "), args
}

// GetMetrikPerbandingan: dioptimalkan jadi SINGLE table scan dengan FILTER clause.
// Versi lama melakukan dua CTE (data_sekarang & data_lalu) yang masing-masing
// melakukan scan terpisah → 2× I/O. Versi baru menscan rentang gabungan satu
// kali dan memisahkan agregasi current vs previous via FILTER (WHERE …).
func GetMetrikPerbandingan(req models.FilterRequest) (models.MetrikResponse, error) {
	whr, filterArgs := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)

	currStart, currEnd, prevStart, prevEnd, err := getPeriodRanges(req.TanggalMulai, req.TanggalSelesai)
	if err != nil {
		return models.MetrikResponse{}, err
	}

	queryArgs := make([]interface{}, 0, 4+len(filterArgs))
	queryArgs = append(queryArgs, currStart, currEnd, prevStart, prevEnd)
	queryArgs = append(queryArgs, filterArgs...)

	sqlQuery := `
		SELECT
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)                                AS gmv_s,
			CAST(ROUND(COALESCE(SUM(quantity::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)) AS BIGINT)     AS qty_s,
			COUNT(DISTINCT product_code_new) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE))                               AS sku_s,
			COUNT(DISTINCT order_number)     FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE))                               AS psn_s,
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0)                                AS gmv_l,
			CAST(ROUND(COALESCE(SUM(quantity::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0)) AS BIGINT)     AS qty_l,
			COUNT(DISTINCT product_code_new) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE))                               AS sku_l,
			COUNT(DISTINCT order_number)     FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE))                               AS psn_l
		FROM marts_ecommerce
		WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
		    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) ` + whr

	var res models.MetrikResponse
	var qtyS, qtyL int64
	var skuS, skuL, psnS, psnL int64
	err = database.DB.QueryRow(sqlQuery, queryArgs...).Scan(
		&res.GMV.Sekarang, &qtyS, &skuS, &psnS,
		&res.GMV.Lalu, &qtyL, &skuL, &psnL,
	)
	if err != nil {
		return res, err
	}
	res.Qty.Sekarang = qtyS
	res.Qty.Lalu = qtyL
	res.SKU.Sekarang = float64(skuS)
	res.SKU.Lalu = float64(skuL)
	res.Pesanan.Sekarang = float64(psnS)
	res.Pesanan.Lalu = float64(psnL)
	return res, nil
}

// GetTrenGrafik: dioptimalkan jadi SINGLE scan dengan FILTER per periode.
// Hasil tetap di-shape jadi paired by-day (current join previous-month).
func GetTrenGrafik(req models.FilterRequest) ([]models.TrenGrafik, error) {
	currStart, currEnd, prevStart, prevEnd, err := getPeriodRanges(req.TanggalMulai, req.TanggalSelesai)
	if err != nil {
		return nil, err
	}

	whr, filterArgs := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)

	queryArgs := make([]interface{}, 0, 4+len(filterArgs))
	queryArgs = append(queryArgs, currStart, currEnd, prevStart, prevEnd)
	queryArgs = append(queryArgs, filterArgs...)

	sqlQuery := `
		WITH periode_sekarang AS (
			SELECT gs::DATE AS date
			FROM generate_series(CAST($1 AS DATE), CAST($2 AS DATE), INTERVAL '1 day') gs
		),
		agg AS (
			SELECT date,
				CAST(SUM(omzet) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)) AS DOUBLE PRECISION) AS gmv_s,
				CAST(ROUND(CAST(SUM(quantity) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)) AS DOUBLE PRECISION)) AS BIGINT) AS qty_s,
				CAST(SUM(omzet) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) AS DOUBLE PRECISION) AS gmv_p,
				CAST(ROUND(CAST(SUM(quantity) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) AS DOUBLE PRECISION)) AS BIGINT) AS qty_p
			FROM marts_ecommerce
			WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
			    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) ` + whr + `
			GROUP BY date
		)
		SELECT 
			COALESCE(CAST(curr.date AS TEXT), '') AS sumbu_x,
			COALESCE(ds.gmv_s, 0.0)               AS periode_sekarang,
			COALESCE(ds.qty_s, 0)                 AS qty_sekarang,
			COALESCE(CAST((curr.date - INTERVAL '1 month')::DATE AS TEXT), '') AS sumbu_x_lalu,
			COALESCE(dp.gmv_p, 0.0)               AS periode_sebelumnya,
			COALESCE(dp.qty_p, 0)                 AS qty_sebelumnya
		FROM periode_sekarang curr
		LEFT JOIN agg ds ON curr.date = ds.date
		LEFT JOIN agg dp ON (curr.date - INTERVAL '1 month')::DATE = dp.date
		ORDER BY curr.date`

	rows, err := database.DB.Query(sqlQuery, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TrenGrafik
	for rows.Next() {
		var t models.TrenGrafik
		if err := rows.Scan(&t.SumbuX, &t.PeriodeSekarang, &t.QtySekarang,
			&t.SumbuXLalu, &t.PeriodeSebelumnya, &t.QtySebelumnya); err == nil {
			list = append(list, t)
		}
	}
	return list, nil
}

// GetPeringkatOutlet: digabung jadi satu scan dengan FILTER untuk current+previous.
func GetPeringkatOutlet(req models.FilterRequest) ([]map[string]interface{}, error) {
	whr, filterArgs := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)

	tglMLalu, tglSLalu, err := getPreviousMonthRange(req.TanggalMulai)
	if err != nil {
		return nil, err
	}

	sqlQuery := `
		SELECT
			COALESCE(NULLIF(TRIM(toko), ''), 'Unknown') AS nama_toko,
			COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)::float8 AS gmv_total,
			CASE
				WHEN COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) = 0 THEN 0.0
				ELSE ((COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)
				     - COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0))
				     / NULLIF(COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0), 0)) * 100.0
			END AS growth
		FROM marts_ecommerce
		WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
		    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) ` + whr + `
		GROUP BY nama_toko
		HAVING COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0) > 0
		    OR COALESCE(SUM(omzet) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) > 0
		ORDER BY gmv_total DESC`

	queryArgs := make([]interface{}, 0, 4+len(filterArgs))
	queryArgs = append(queryArgs, req.TanggalMulai, req.TanggalSelesai, tglMLalu, tglSLalu)
	queryArgs = append(queryArgs, filterArgs...)

	rows, err := database.DB.Query(sqlQuery, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var namaToko string
		var gmvTotal, growth float64
		if err := rows.Scan(&namaToko, &gmvTotal, &growth); err == nil {
			result = append(result, map[string]interface{}{
				"nama_toko": namaToko,
				"gmv_total": gmvTotal,
				"growth":    growth,
			})
		}
	}
	return result, nil
}

var (
	martsColumnCache   map[string]bool
	martsColumnCacheMu sync.RWMutex
)

func getMartsColumns() map[string]bool {
	martsColumnCacheMu.RLock()
	if martsColumnCache != nil {
		c := martsColumnCache
		martsColumnCacheMu.RUnlock()
		return c
	}
	martsColumnCacheMu.RUnlock()

	out := map[string]bool{}
	rows, err := database.DB.Query(`
		SELECT LOWER(column_name)
		FROM information_schema.columns
		WHERE table_name = 'marts_ecommerce'
	`)
	if err != nil {
		fmt.Printf("Warning: gagal introspeksi kolom marts_ecommerce: %v\n", err)
		martsColumnCacheMu.Lock()
		martsColumnCache = out
		martsColumnCacheMu.Unlock()
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			out[name] = true
		}
	}
	martsColumnCacheMu.Lock()
	martsColumnCache = out
	martsColumnCacheMu.Unlock()
	return out
}

func pickStatusColumn(candidates ...string) string {
	cols := getMartsColumns()
	for _, c := range candidates {
		if cols[strings.ToLower(c)] {
			return fmt.Sprintf("COALESCE(NULLIF(TRIM(%s::text), ''), '-')", c)
		}
	}
	return "'-'::text"
}

func GetStatusLogistik(req models.FilterRequest) ([]models.StatusLogistik, error) {
	whr, filterArgs := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 3)

	queryArgs := make([]interface{}, 0, 4+len(filterArgs))
	queryArgs = append(queryArgs, req.TanggalMulai, req.TanggalSelesai)
	queryArgs = append(queryArgs, filterArgs...)

	exprWms := pickStatusColumn("status_wms", "wms_status")
	expr3pl := pickStatusColumn("status_3pl", "status_toko", "toko_status", "status_3PL")
	exprFix := pickStatusColumn("status_fix", "status_fixed", "status_standard", "standard_status", "status")
	exprStdReturn := pickStatusColumn("std_return", "alasan_return", "return_reason", "alasan_retur")

	searchClause := ""
	limitClause := " LIMIT 5000"
	if s := strings.TrimSpace(req.Search); s != "" {
		nextIdx := 3 + len(filterArgs)
		like := s + "%"
		searchClause = fmt.Sprintf(` AND (
			order_number ILIKE $%d
			OR toko ILIKE $%d
			OR kanal ILIKE $%d
		)`, nextIdx, nextIdx, nextIdx)
		queryArgs = append(queryArgs, like)
		limitClause = " LIMIT 50"
	}

	sqlQuery := `
		SELECT
			COALESCE(TO_CHAR(date, 'YYYY-MM-DD'), '') AS tanggal,
			COALESCE(NULLIF(TRIM(order_number), ''), '-') AS order_number,
			COALESCE(NULLIF(TRIM(toko), ''), 'Unknown Store') AS nama_toko,
			COALESCE(NULLIF(TRIM(kanal), ''), 'Unknown Kanal') AS kanal,
			` + exprWms + ` AS status_wms,
			` + expr3pl + ` AS status_3pl,
			` + exprFix + ` AS status_fix,
			COALESCE(omzet, 0)::float8 AS omzet,
			` + exprStdReturn + ` AS std_return
		FROM marts_ecommerce
		WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE) ` + whr + searchClause + limitClause

	rows, err := database.DB.Query(sqlQuery, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]models.StatusLogistik, 0, 4096)
	for rows.Next() {
		var l models.StatusLogistik
		if err := rows.Scan(&l.Tanggal, &l.OrderNumber, &l.NamaToko, &l.Kanal, &l.StatusWms, &l.Status3pl, &l.StatusFix, &l.Omzet, &l.StdReturn); err == nil {
			list = append(list, l)
		}
	}
	return list, nil
}

func GetDropdownOptions(channelSel, kanalSel, namaTokoSel, kategoriSel, productSel, brandSel, teamSel []string) (models.OptionsResponse, error) {
	res := models.OptionsResponse{
		Channel:     []string{},
		Kanal:       []string{},
		NamaToko:    []string{},
		Kategori:    []string{},
		ProductName: []string{},
		Brand:       []string{},
		Team:        []string{},
	}

	cClean := cleanMulti(channelSel)
	kClean := cleanMulti(kanalSel)
	tClean := cleanMulti(namaTokoSel)
	katClean := cleanMulti(kategoriSel)
	prdClean := cleanMulti(productSel)
	brdClean := cleanMulti(brandSel)
	tmClean := cleanMulti(teamSel)

	buildScopedDistinct := func(col string) (string, []interface{}) {
		q := fmt.Sprintf(`
			SELECT DISTINCT TRIM(%s)
			FROM marts_ecommerce
			WHERE %s IS NOT NULL AND TRIM(%s) != ''`, col, col, col)
		var args []interface{}
		idx := 1
		addIn := func(condCol string, vals []string) {
			if condCol == col || len(vals) == 0 {
				return
			}
			placeholders := make([]string, 0, len(vals))
			for _, v := range vals {
				placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
				args = append(args, strings.ToLower(v))
				idx++
			}
			q += fmt.Sprintf(` AND TRIM(LOWER(%s)) IN (%s)`, condCol, strings.Join(placeholders, ", "))
		}
		addChannel := func(vals []string) {
			if col == "channel" || len(vals) == 0 {
				return
			}
			var parts []string
			for _, v := range vals {
				l := strings.ToLower(v)
				parts = append(parts, fmt.Sprintf("LOWER(channel) LIKE $%d", idx))
				switch {
				case strings.Contains(l, "online"):
					args = append(args, "%online%")
				case strings.Contains(l, "offline"):
					args = append(args, "%offline%")
				default:
					args = append(args, "%"+l+"%")
				}
				idx++
			}
			q += " AND (" + strings.Join(parts, " OR ") + ")"
		}
		addChannel(cClean)
		addIn("kanal", kClean)
		addIn("toko", tClean)
		addIn("kategori", katClean)
		addIn("product_name", prdClean)
		addIn("brand", brdClean)
		addIn("team", tmClean)
		q += fmt.Sprintf(` ORDER BY TRIM(%s)`, col)
		return q, args
	}

	loadDistinct := func(col string) []string {
		list := []string{}
		q, args := buildScopedDistinct(col)
		rows, err := database.DB.Query(q, args...)
		if err != nil || rows == nil {
			return list
		}
		defer rows.Close()
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err == nil {
				vStr := strings.TrimSpace(v)
				if vStr != "" && !strings.HasPrefix(strings.ToLower(vStr), "semua") {
					list = append(list, vStr)
				}
			}
		}
		return list
	}

	columns := []struct {
		col    string
		assign func([]string)
	}{
		{"channel", func(v []string) { res.Channel = v }},
		{"kanal", func(v []string) { res.Kanal = v }},
		{"toko", func(v []string) { res.NamaToko = v }},
		{"kategori", func(v []string) { res.Kategori = v }},
		{"product_name", func(v []string) { res.ProductName = v }},
		{"brand", func(v []string) { res.Brand = v }},
		{"team", func(v []string) { res.Team = v }},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, c := range columns {
		wg.Add(1)
		go func(col string, assign func([]string)) {
			defer wg.Done()
			v := loadDistinct(col)
			mu.Lock()
			assign(v)
			mu.Unlock()
		}(c.col, c.assign)
	}
	wg.Wait()

	return res, nil
}

// GetRingkasanHarian: dipertahankan struktur 3-query (total, per-kanal, per-toko)
// karena per-kanal & per-toko butuh GROUP BY berbeda. Tiap query sudah pakai
// FILTER tunggal pass (current + previous bersamaan).
func GetRingkasanHarian(req models.FilterRequest) (models.RingkasanResponse, error) {
	var res models.RingkasanResponse

	currStart, currEnd, prevStart, prevEnd, err := getPeriodRanges(req.TanggalMulai, req.TanggalSelesai)
	if err != nil {
		return res, err
	}
	res.PeriodeSekarangMulai = currStart
	res.PeriodeSekarangSelesai = currEnd
	res.PeriodeLaluMulai = prevStart
	res.PeriodeLaluSelesai = prevEnd

	whrAll, argsAll := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)
	totalQuery := `
		SELECT
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0) AS omset_now,
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) AS omset_prev,
			CAST(ROUND(COALESCE(SUM(quantity::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0)) AS BIGINT) AS qty_now,
			CAST(ROUND(COALESCE(SUM(quantity::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0)) AS BIGINT) AS qty_prev
		FROM marts_ecommerce
		WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
		    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) ` + whrAll
	totalArgs := append([]interface{}{currStart, currEnd, prevStart, prevEnd}, argsAll...)
	if err := database.DB.QueryRow(totalQuery, totalArgs...).Scan(
		&res.TotalOmset, &res.TotalOmsetLalu, &res.TotalQty, &res.TotalQtyLalu,
	); err != nil {
		return res, err
	}

	whrK, argsK := buildFilterClausesExt(req.Channel, nil, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)
	kanalQuery := `
		SELECT
			COALESCE(NULLIF(TRIM(kanal), ''), 'Unknown') AS kanal,
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0) AS omset_now,
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) AS omset_prev
		FROM marts_ecommerce
		WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
		    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) ` + whrK + `
		GROUP BY kanal
		HAVING COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0) > 0
		    OR COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) > 0
		ORDER BY omset_now DESC`
	kanalArgs := append([]interface{}{currStart, currEnd, prevStart, prevEnd}, argsK...)
	rowsK, err := database.DB.Query(kanalQuery, kanalArgs...)
	if err != nil {
		return res, err
	}
	defer rowsK.Close()
	for rowsK.Next() {
		var k models.KanalBreakdown
		if err := rowsK.Scan(&k.Kanal, &k.OmsetSekarang, &k.OmsetLalu); err == nil {
			res.PerKanal = append(res.PerKanal, k)
		}
	}

	whrT, argsT := buildFilterClausesExt(req.Channel, req.Kanal, req.NamaToko, req.Kategori, req.ProductName, req.Brand, req.Team, 5)
	tokoQuery := `
		SELECT
			COALESCE(NULLIF(TRIM(toko), ''), 'Unknown') AS toko,
			COALESCE(MAX(NULLIF(TRIM(kanal), '')), '') AS kanal,
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0) AS omset_now,
			COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) AS omset_prev
		FROM marts_ecommerce
		WHERE (date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)
		    OR date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)) ` + whrT + `
		GROUP BY toko
		HAVING COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($1 AS DATE) AND CAST($2 AS DATE)), 0) > 0
		    OR COALESCE(SUM(omzet::NUMERIC) FILTER (WHERE date BETWEEN CAST($3 AS DATE) AND CAST($4 AS DATE)), 0) > 0`
	tokoArgs := append([]interface{}{currStart, currEnd, prevStart, prevEnd}, argsT...)
	rowsT, err := database.DB.Query(tokoQuery, tokoArgs...)
	if err != nil {
		return res, err
	}
	defer rowsT.Close()

	var topGrowth, bottomGrowth *models.TokoBreakdown
	var topPct, bottomPct float64
	for rowsT.Next() {
		var t models.TokoBreakdown
		if err := rowsT.Scan(&t.Toko, &t.Kanal, &t.OmsetSekarang, &t.OmsetLalu); err != nil {
			continue
		}
		if t.OmsetLalu <= 0 {
			continue
		}
		pct := ((t.OmsetSekarang - t.OmsetLalu) / t.OmsetLalu) * 100
		t.Growth = pct
		tCopy := t
		if topGrowth == nil || pct > topPct {
			topGrowth = &tCopy
			topPct = pct
		}
		if bottomGrowth == nil || pct < bottomPct {
			bottomGrowth = &tCopy
			bottomPct = pct
		}
	}
	if topGrowth != nil && bottomGrowth != nil && topGrowth.Toko == bottomGrowth.Toko {
		bottomGrowth = nil
	}
	res.TokoTopGrowth = topGrowth
	res.TokoBottomGrowth = bottomGrowth

	return res, nil
}
