package site

import (
	"errors"
	"testing"
)

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

func TestValidateForEventSourceAllowsServerWithoutNavigation(t *testing.T) {
	config := validSiteConfig()

	if err := ValidateForEventSource(config, config.SiteCode, "production", 0, "", SourceServer); err != nil {
		t.Fatalf("ValidateForEventSource() error = %v", err)
	}
}

func TestValidateForEventSourceKeepsWebRules(t *testing.T) {
	config := validSiteConfig()

	if err := ValidateForEventSource(config, config.SiteCode, "production", config.TokenVersion, "", ""); !errors.Is(err, ErrDomainNotAllowed) {
		t.Fatalf("ValidateForEventSource() error = %v, want ErrDomainNotAllowed", err)
	}
	if err := ValidateForEventSource(config, config.SiteCode, "production", config.TokenVersion, "https://example.com", "mobile"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ValidateForEventSource() error = %v, want ErrInvalidSource", err)
	}
}

func TestValidateTimezoneRequiresValidIANAName(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		wantErr  bool
	}{
		{name: "valida", timezone: "America/Argentina/Buenos_Aires"},
		{name: "ausente", timezone: "", wantErr: true},
		{name: "invalida", timezone: "Mars/Olympus", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateTimezone(SiteConfig{Timezone: test.timezone})
			if test.wantErr && !errors.Is(err, ErrTimezoneUnresolved) {
				t.Fatalf("ValidateTimezone() error = %v, want ErrTimezoneUnresolved", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidateTimezone() error = %v", err)
			}
		})
	}
}

func validSiteConfig() SiteConfig {
	return SiteConfig{
		SiteCode:        "site-code",
		TenantID:        "tenant-1",
		SiteID:          "site-1",
		Status:          "active",
		TrackingEnabled: true,
		AllowedDomains:  []string{"example.com"},
		TokenVersion:    1,
		SampleRate:      1,
		SchemaVersion:   2,
	}
}
