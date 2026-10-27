package config

import "testing"

func TestIsProduction(t *testing.T) {
	cases := map[string]bool{
		"production":  true,
		"Production":  true,
		"prod":        true,
		"development": false,
		"staging":     false,
		"":            false,
	}
	for env, want := range cases {
		if got := IsProduction(env); got != want {
			t.Errorf("IsProduction(%q)=%v want %v", env, got, want)
		}
	}
}
