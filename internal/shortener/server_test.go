package shortener

import "testing"

func TestValidHTTPURL(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"https://example.com/path?q=1", true},
		{"http://localhost:8080/path", true},
		{"ftp://example.com/file", false},
		{"https://user:password@example.com/secret", false},
		{"not a URL", false},
		{"/relative", false},
		{"https://example.com/" + string(make([]byte, maxURLLength)), false},
	}
	for _, test := range tests {
		if got := validHTTPURL(test.value); got != test.valid {
			t.Errorf("validHTTPURL(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestCacheKey(t *testing.T) {
	if got := cacheKey(42); got != "url:42" {
		t.Fatalf("cacheKey(42) = %q", got)
	}
}

func TestConfigFromEnvRequiresDataStores(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("missing data store configuration was accepted")
	}

	t.Setenv("DATABASE_URL", "postgresql://localhost/shortener")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if config.CacheTTL <= 0 || config.KafkaBroker == "" {
		t.Fatalf("invalid defaults: %#v", config)
	}
}
