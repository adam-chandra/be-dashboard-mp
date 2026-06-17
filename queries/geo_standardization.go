package queries

import (
	"regexp"
	"strings"
)

var provinsiCanonical = map[string]string{
	"ACEH":                          "Aceh",
	"SUMATERA UTARA":                "Sumatera Utara",
	"SUMATRA UTARA":                 "Sumatera Utara",
	"SUMUT":                         "Sumatera Utara",
	"SUMATERA BARAT":                "Sumatera Barat",
	"SUMATRA BARAT":                 "Sumatera Barat",
	"SUMBAR":                        "Sumatera Barat",
	"RIAU":                          "Riau",
	"KEPULAUAN RIAU":                "Kepulauan Riau",
	"KEP. RIAU":                     "Kepulauan Riau",
	"KEPRI":                         "Kepulauan Riau",
	"JAMBI":                         "Jambi",
	"SUMATERA SELATAN":              "Sumatera Selatan",
	"SUMATRA SELATAN":               "Sumatera Selatan",
	"SUMSEL":                        "Sumatera Selatan",
	"KEPULAUAN BANGKA BELITUNG":     "Kepulauan Bangka Belitung",
	"KEP. BANGKA BELITUNG":          "Kepulauan Bangka Belitung",
	"BANGKA BELITUNG":               "Kepulauan Bangka Belitung",
	"BANGKA-BELITUNG":               "Kepulauan Bangka Belitung",
	"BABEL":                         "Kepulauan Bangka Belitung",
	"BENGKULU":                      "Bengkulu",
	"LAMPUNG":                       "Lampung",
	"DKI JAKARTA":                   "DKI Jakarta",
	"DKI":                           "DKI Jakarta",
	"JAKARTA":                       "DKI Jakarta",
	"DAERAH KHUSUS IBUKOTA JAKARTA": "DKI Jakarta",
	"BANTEN":                        "Banten",
	"JAWA BARAT":                    "Jawa Barat",
	"JABAR":                         "Jawa Barat",
	"JAWA TENGAH":                   "Jawa Tengah",
	"JATENG":                        "Jawa Tengah",
	"DI YOGYAKARTA":                 "DI Yogyakarta",
	"DAERAH ISTIMEWA YOGYAKARTA":    "DI Yogyakarta",
	"YOGYAKARTA":                    "DI Yogyakarta",
	"DIY":                           "DI Yogyakarta",
	"JAWA TIMUR":                    "Jawa Timur",
	"JATIM":                         "Jawa Timur",
	"BALI":                          "Bali",
	"NUSA TENGGARA BARAT":           "Nusa Tenggara Barat",
	"NTB":                           "Nusa Tenggara Barat",
	"NUSA TENGGARA TIMUR":           "Nusa Tenggara Timur",
	"NTT":                           "Nusa Tenggara Timur",
	"KALIMANTAN BARAT":              "Kalimantan Barat",
	"KALBAR":                        "Kalimantan Barat",
	"KALIMANTAN TENGAH":             "Kalimantan Tengah",
	"KALTENG":                       "Kalimantan Tengah",
	"KALIMANTAN SELATAN":            "Kalimantan Selatan",
	"KALSEL":                        "Kalimantan Selatan",
	"KALIMANTAN TIMUR":              "Kalimantan Timur",
	"KALTIM":                        "Kalimantan Timur",
	"KALIMANTAN UTARA":              "Kalimantan Utara",
	"KALTARA":                       "Kalimantan Utara",
	"SULAWESI UTARA":                "Sulawesi Utara",
	"SULUT":                         "Sulawesi Utara",
	"GORONTALO":                     "Gorontalo",
	"SULAWESI TENGAH":               "Sulawesi Tengah",
	"SULTENG":                       "Sulawesi Tengah",
	"SULAWESI BARAT":                "Sulawesi Barat",
	"SULBAR":                        "Sulawesi Barat",
	"SULAWESI SELATAN":              "Sulawesi Selatan",
	"SULSEL":                        "Sulawesi Selatan",
	"SULAWESI TENGGARA":             "Sulawesi Tenggara",
	"SULTRA":                        "Sulawesi Tenggara",
	"MALUKU":                        "Maluku",
	"MALUKU UTARA":                  "Maluku Utara",
	"PAPUA":                         "Papua",
	"PAPUA BARAT":                   "Papua Barat",
	"IRIAN JAYA":                    "Papua",
	"IRIAN JAYA BARAT":              "Papua Barat",
	"PAPUA BARAT DAYA":              "Papua Barat Daya",
	"PAPUA TENGAH":                  "Papua Tengah",
	"PAPUA PEGUNUNGAN":              "Papua Pegunungan",
	"PAPUA SELATAN":                 "Papua Selatan",
}

var cityToProvinceMap = map[string]string{
	"FAKFAK":             "Papua Barat",
	"FAK FAK":            "Papua Barat",
	"KAIMANA":            "Papua Barat",
	"MANOKWARI":          "Papua Barat",
	"MANOKWARI SELATAN":  "Papua Barat",
	"TELUK BINTUNI":      "Papua Barat",
	"TELUK WONDAMA":      "Papua Barat",
	"PEGUNUNGAN ARFAK":   "Papua Barat",
	"SORONG":             "Papua Barat Daya",
	"KOTA SORONG":        "Papua Barat Daya",
	"SORONG SELATAN":     "Papua Barat Daya",
	"RAJA AMPAT":         "Papua Barat Daya",
	"MAYBRAT":            "Papua Barat Daya",
	"TAMBRAUW":           "Papua Barat Daya",
	"JAYAPURA":           "Papua",
	"KOTA JAYAPURA":      "Papua",
	"KEEROM":             "Papua",
	"MAMBERAMO RAYA":     "Papua",
	"SARMI":              "Papua",
	"SUPIORI":            "Papua",
	"BIAK":               "Papua",
	"BIAK NUMFOR":        "Papua",
	"KEPULAUAN YAPEN":    "Papua",
	"WAROPEN":            "Papua",
	"NABIRE":             "Papua Tengah",
	"PUNCAK":             "Papua Tengah",
	"PUNCAK JAYA":        "Papua Tengah",
	"MIMIKA":             "Papua Tengah",
	"TIMIKA":             "Papua Tengah",
	"DOGIYAI":            "Papua Tengah",
	"DEIYAI":             "Papua Tengah",
	"PANIAI":             "Papua Tengah",
	"INTAN JAYA":         "Papua Tengah",
	"JAYAWIJAYA":         "Papua Pegunungan",
	"LANNY JAYA":         "Papua Pegunungan",
	"MAMBERAMO TENGAH":   "Papua Pegunungan",
	"NDUGA":              "Papua Pegunungan",
	"PEGUNUNGAN BINTANG": "Papua Pegunungan",
	"TOLIKARA":           "Papua Pegunungan",
	"YAHUKIMO":           "Papua Pegunungan",
	"YALIMO":             "Papua Pegunungan",
	"WAMENA":             "Papua Pegunungan",
	"MERAUKE":            "Papua Selatan",
	"BOVEN DIGOEL":       "Papua Selatan",
	"MAPPI":              "Papua Selatan",
	"ASMAT":              "Papua Selatan",
}

var fakfakPattern = regexp.MustCompile(`(?i)\bfak[\s\-]?fak\b`)

var (
	rePrefixWilayah  = regexp.MustCompile(`(?i)^(kotamadya|kotamdy|kabupaten|kota|kab\.?|daerah)\s+`)
	reSuffixWilayah  = regexp.MustCompile(`(?i)\s+(kotamadya|kotamdy|kabupaten|kota|kab)\.?$`)
	reMultiWhitespc  = regexp.MustCompile(`\s+`)
	rePrefixProvinsi = regexp.MustCompile(`(?i)^(provinsi|prov\.?)\s+`)
)

func normalizeProvinsiName(s string) string {
	if s == "" {
		return ""
	}
	u := strings.ToUpper(strings.TrimSpace(s))
	u = rePrefixProvinsi.ReplaceAllString(u, "")
	u = reMultiWhitespc.ReplaceAllString(u, " ")
	u = strings.TrimSpace(u)
	if v, ok := provinsiCanonical[u]; ok {
		return v
	}
	collapsed := strings.ReplaceAll(u, "-", " ")
	collapsed = reMultiWhitespc.ReplaceAllString(collapsed, " ")
	collapsed = strings.TrimSpace(collapsed)
	if v, ok := provinsiCanonical[collapsed]; ok {
		return v
	}
	return ""
}

func provinceFromCity(s string) string {
	if s == "" {
		return ""
	}
	u := strings.ToUpper(strings.TrimSpace(s))
	if fakfakPattern.MatchString(u) {
		return "Papua Barat"
	}
	u = rePrefixWilayah.ReplaceAllString(u, "")
	u = reSuffixWilayah.ReplaceAllString(u, "")
	u = reMultiWhitespc.ReplaceAllString(u, " ")
	u = strings.TrimSpace(u)
	if v, ok := cityToProvinceMap[u]; ok {
		return v
	}
	collapsed := strings.ReplaceAll(u, "-", " ")
	collapsed = reMultiWhitespc.ReplaceAllString(collapsed, " ")
	collapsed = strings.TrimSpace(collapsed)
	if v, ok := cityToProvinceMap[collapsed]; ok {
		return v
	}
	return ""
}

func resolveProvinsi(rawProvince, rawCity string) string {
	if v := normalizeProvinsiName(rawProvince); v != "" {
		return v
	}
	return provinceFromCity(rawCity)
}
