package orchestrator

import "sort"

func percentile(data []int64, p int) int64 {
	sort.Slice(data, func(i, j int) bool {
		return data[i] < data[j]
	})

	index := (len(data) - 1) * p / 100
	return data[index]
}

func stddev(data []int64, mean int64) float64 {
	var variance float64
	for _, v := range data {
		diff := float64(v - mean)
		variance += diff * diff
	}
	variance /= float64(len(data))
	return sqrt(variance)
}

func sqrt(value float64) float64 {
	if value <= 0 {
		return 0
	}

	z := value
	for i := 0; i < 10; i++ {
		z -= (z*z - value) / (2 * z)
	}
	return z
}

func meanInt64(data []int64) float64 {
	var sum int64
	for _, v := range data {
		sum += v
	}
	return float64(sum) / float64(len(data))
}

func jitter(data []int64) float64 {
	if len(data) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(data); i++ {
		diff := data[i] - data[i-1]
		if diff < 0 {
			diff = -diff
		}
		total += float64(diff)
	}
	return total / float64(len(data)-1)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
