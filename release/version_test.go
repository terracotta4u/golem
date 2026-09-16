package release

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"0.2.0", "v0.1.0", true},
		{"v0.1.0", "0.1.0", false},
		{"v0.1.0", "0.2.0", false},
		{"v0.2.0", "dev", false},
		{"v0.2.0", "", false},
		{"v0.2.0", "none", false},
		{"not-a-version", "0.1.0", false},
		{"0.1.0", "not-a-version", false},
	}
	for _, tc := range cases {
		got := Newer(tc.latest, tc.current)
		if got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestDisplay(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "dev"},
		{"dev", "dev"},
		{"v0.1.0", "0.1.0"},
		{"0.1.0", "0.1.0"},
	}
	for _, tc := range cases {
		got := Display(tc.in)
		if got != tc.want {
			t.Errorf("Display(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
