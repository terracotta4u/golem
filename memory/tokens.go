package memory

import "unicode/utf8"

type TokenEstimator interface {
	Estimate(text string) int
}

type ApproxTokenEstimator struct{}

func (ApproxTokenEstimator) Estimate(s string) int {
	if s == "" {
		return 0
	}
	n := utf8.RuneCountInString(s) / 4
	if n < 1 {
		return 1
	}
	return n
}
