package validator

const (
	B  = 1
	KB = 1 << (10 * iota)
	MB
	GB
)

func GbToB(gb uint64) float64 {
	return float64(gb) * GB
}

func BToGb(b uint64) float64 {
	return float64(b) / GB
}
