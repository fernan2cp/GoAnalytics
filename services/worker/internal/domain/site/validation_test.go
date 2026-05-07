package site

import "testing"

func TestAllowsOriginNormalizesSchemeWWWAndPort(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed []string
		want    bool
	}{
		{
			name:    "origin local con esquema matchea dominio con puerto",
			origin:  "http://127.0.0.1:5173",
			allowed: []string{"127.0.0.1:5173"},
			want:    true,
		},
		{
			name:    "origin https matchea dominio sin esquema",
			origin:  "https://example.com",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "www se ignora en origin",
			origin:  "https://www.example.com",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "www se ignora en allowed domain",
			origin:  "https://example.com",
			allowed: []string{"www.example.com"},
			want:    true,
		},
		{
			name:    "puerto distinto no matchea",
			origin:  "http://127.0.0.1:5174",
			allowed: []string{"127.0.0.1:5173"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SiteConfig{AllowedDomains: tt.allowed}

			got := AllowsOrigin(config, tt.origin)

			if got != tt.want {
				t.Fatalf("AllowsOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}
