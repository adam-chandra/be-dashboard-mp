package models

import (
	"encoding/json"
	"sort"
	"strings"
)

type MultiString []string

func (m *MultiString) UnmarshalJSON(data []byte) error {
	t := strings.TrimSpace(string(data))
	if t == "" || t == "null" {
		*m = nil
		return nil
	}
	if t[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*m = MultiString(arr)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*m = nil
	} else {
		*m = MultiString{s}
	}
	return nil
}

func (m MultiString) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(m))
}

func (m MultiString) Clean() MultiString {
	if len(m) == 0 {
		return MultiString{}
	}
	out := make(MultiString, 0, len(m))
	seen := make(map[string]struct{}, len(m))
	for _, v := range m {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(t), "semua") {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

type FilterRequest struct {
	TanggalMulai   string      `json:"tanggal_mulai"`
	TanggalSelesai string      `json:"tanggal_selesai"`
	Channel        MultiString `json:"channel"`
	NamaToko       MultiString `json:"nama_toko"`
	Kanal          MultiString `json:"kanal"`
	Kategori       MultiString `json:"kategori"`
	ProductName    MultiString `json:"product_name"`
	Brand          MultiString `json:"brand"`
	Team           MultiString `json:"team"`
	Search         string      `json:"search"`
}

func (r *FilterRequest) Normalize() {
	r.Channel = r.Channel.Clean()
	r.Kanal = r.Kanal.Clean()
	r.NamaToko = r.NamaToko.Clean()
	r.Kategori = r.Kategori.Clean()
	r.ProductName = r.ProductName.Clean()
	r.Brand = r.Brand.Clean()
	r.Team = r.Team.Clean()
	r.Search = strings.TrimSpace(r.Search)
}

func NormalizeOptionParam(values []string) MultiString {
	return MultiString(values).Clean()
}

type MetrikTuple struct {
	Sekarang float64 `json:"sekarang"`
	Lalu     float64 `json:"lalu"`
}

type QtyTuple struct {
	Sekarang int64 `json:"sekarang"`
	Lalu     int64 `json:"lalu"`
}

type MetrikResponse struct {
	GMV     MetrikTuple `json:"gmv"`
	Qty     QtyTuple    `json:"qty"`
	SKU     MetrikTuple `json:"sku"`
	Pesanan MetrikTuple `json:"pesanan"`
}

type TrenGrafik struct {
	SumbuX            string  `json:"sumbu_x"`
	PeriodeSekarang   float64 `json:"periode_sekarang"`
	QtySekarang       int64   `json:"qty_sekarang"`
	SumbuXLalu        string  `json:"sumbu_x_lalu"`
	PeriodeSebelumnya float64 `json:"periode_sebelumnya"`
	QtySebelumnya     int64   `json:"qty_sebelumnya"`
}

type PeringkatToko struct {
	Peringkat          int     `json:"peringkat"`
	NamaToko           string  `json:"nama_toko"`
	GMVTotal           float64 `json:"gmv_total"`
	PersentasePerforma float64 `json:"persentase_performa"`
}

type StatusLogistik struct {
	Tanggal     string  `json:"tanggal"`
	OrderNumber string  `json:"order_number"`
	NamaToko    string  `json:"nama_toko"`
	Kanal       string  `json:"kanal"`
	StatusWms   string  `json:"status_wms"`
	Status3pl   string  `json:"status_3pl"`
	StatusFix   string  `json:"status_fix"`
	Omzet       float64 `json:"omzet"`
	StdReturn   string  `json:"std_return"`
}

type OptionsResponse struct {
	Channel     []string `json:"channel"`
	Kanal       []string `json:"kanal"`
	NamaToko    []string `json:"nama_toko"`
	Kategori    []string `json:"kategori"`
	ProductName []string `json:"product_name"`
	Brand       []string `json:"brand"`
	Team        []string `json:"team"`
}

type KanalBreakdown struct {
	Kanal         string  `json:"kanal"`
	OmsetSekarang float64 `json:"omset_sekarang"`
	OmsetLalu     float64 `json:"omset_lalu"`
}

type TokoBreakdown struct {
	Toko          string  `json:"toko"`
	Kanal         string  `json:"kanal"`
	OmsetSekarang float64 `json:"omset_sekarang"`
	OmsetLalu     float64 `json:"omset_lalu"`
	Growth        float64 `json:"growth"`
}

type RingkasanResponse struct {
	PeriodeSekarangMulai   string           `json:"periode_sekarang_mulai"`
	PeriodeSekarangSelesai string           `json:"periode_sekarang_selesai"`
	PeriodeLaluMulai       string           `json:"periode_lalu_mulai"`
	PeriodeLaluSelesai     string           `json:"periode_lalu_selesai"`
	TotalOmset             float64          `json:"total_omset"`
	TotalOmsetLalu         float64          `json:"total_omset_lalu"`
	TotalQty               int64            `json:"total_qty"`
	TotalQtyLalu           int64            `json:"total_qty_lalu"`
	PerKanal               []KanalBreakdown `json:"per_kanal"`
	TokoTopGrowth          *TokoBreakdown   `json:"toko_top_growth"`
	TokoBottomGrowth       *TokoBreakdown   `json:"toko_bottom_growth"`
}

type ContribItem struct {
	Label string  `json:"label"`
	Omset float64 `json:"omset"`
	Qty   int64   `json:"qty"`
}

type GeoItem struct {
	Wilayah string  `json:"wilayah"`
	Omset   float64 `json:"omset"`
}

type GeoMappingStats struct {
	MappedCount   int64 `json:"mapped_count"`
	UnmappedCount int64 `json:"unmapped_count"`
}

type HeroProduct struct {
	ProductName string  `json:"product_name"`
	Brand       string  `json:"brand"`
	Kategori    string  `json:"kategori"`
	GMV         float64 `json:"gmv"`
	Share       float64 `json:"share"`
}

type NPLSeries struct {
	Kode           string  `json:"kode"`
	ProductName    string  `json:"product_name"`
	Day30          float64 `json:"day_30"`
	Day60          float64 `json:"day_60"`
	Day90          float64 `json:"day_90"`
	QtyD30         int64   `json:"qty_d30"`
	QtyD60         int64   `json:"qty_d60"`
	QtyD90         int64   `json:"qty_d90"`
	Sampel         int64   `json:"sampel"`
	Found          bool    `json:"found"`
	AutoDiscovered bool    `json:"auto_discovered"`
}

type LeaderboardItem struct {
	Peringkat   int     `json:"peringkat"`
	ProductName string  `json:"product_name"`
	Brand       string  `json:"brand"`
	GMV         float64 `json:"gmv"`
	GMVLalu     float64 `json:"gmv_lalu"`
	Growth      float64 `json:"growth"`
	AOV         float64 `json:"aov"`
	ASP         float64 `json:"asp"`
}

type SalesPerformanceResponse struct {
	PeriodeSekarangMulai   string            `json:"periode_sekarang_mulai"`
	PeriodeSekarangSelesai string            `json:"periode_sekarang_selesai"`
	PeriodeLaluMulai       string            `json:"periode_lalu_mulai"`
	PeriodeLaluSelesai     string            `json:"periode_lalu_selesai"`
	TotalOmset             float64           `json:"total_omset"`
	ContribByBrand         []ContribItem     `json:"contrib_by_brand"`
	ContribByKanal         []ContribItem     `json:"contrib_by_kanal"`
	ContribByKategori      []ContribItem     `json:"contrib_by_kategori"`
	TopProvinsi            []GeoItem         `json:"top_provinsi"`
	TopKota                []GeoItem         `json:"top_kota"`
	ProvinsiMappingStats   GeoMappingStats   `json:"provinsi_mapping_stats"`
	HeroProducts           []HeroProduct     `json:"hero_products"`
	NPLSeries              []NPLSeries       `json:"npl_series"`
	Leaderboard            []LeaderboardItem `json:"leaderboard"`
}
