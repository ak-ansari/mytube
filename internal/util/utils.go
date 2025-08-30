package util

type Quality struct {
	Height    int
	Width     int
	Label     string
	Bandwidth int
}

var Sizes = []Quality{
	{Label: "240p", Bandwidth: 400_000, Width: 426, Height: 240},      // ~0.4 Mbps
	{Label: "360p", Bandwidth: 800_000, Width: 640, Height: 360},      // ~0.8 Mbps
	{Label: "480p", Bandwidth: 1_400_000, Width: 854, Height: 480},    // ~1.4 Mbps
	{Label: "720p", Bandwidth: 2_800_000, Width: 1280, Height: 720},   // ~2.8 Mbps
	{Label: "1080p", Bandwidth: 5_000_000, Width: 1920, Height: 1080}, // ~5 Mbps
}

func GetQualityMap() map[string]Quality {
	qualityMap := make(map[string]Quality)

	for _, q := range Sizes {
		qualityMap[q.Label] = q
	}
	return qualityMap
}
