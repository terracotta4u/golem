package memory

import "testing"

func TestApproxTokenEstimator(t *testing.T) {
	var e ApproxTokenEstimator
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"abc", 1},
		{"abcd", 1},
		{"abcde", 1},
		{"abcdefgh", 2},
	}
	for _, tc := range cases {
		if got := e.Estimate(tc.in); got != tc.want {
			t.Errorf("Estimate(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
