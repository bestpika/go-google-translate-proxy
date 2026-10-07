package main

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	for _, tt := range []struct {
		name      string
		file, env map[string]string
		want      config
	}{
		{"defaults", nil, nil, config{"9009", defaultGoogleURL, defaultGoogleAPIKey}},
		{"file", map[string]string{"PORT": "9090", "GOOGLE_TRANSLATE_URL": "http://example.test/path", "GOOGLE_TRANSLATE_API_KEY": "file-key"}, nil, config{"9090", "http://example.test/path", "file-key"}},
		{"environment", map[string]string{"PORT": "9090", "GOOGLE_TRANSLATE_API_KEY": "file-key"}, map[string]string{"PORT": " 7070 ", "GOOGLE_TRANSLATE_API_KEY": " env-key "}, config{"7070", defaultGoogleURL, "env-key"}},
		{"empty environment", map[string]string{"PORT": "9090"}, map[string]string{"PORT": ""}, config{"9090", defaultGoogleURL, defaultGoogleAPIKey}},
		{"blank environment", map[string]string{"PORT": "9090"}, map[string]string{"PORT": " "}, config{defaultPort, defaultGoogleURL, defaultGoogleAPIKey}},
		{"maximum port", map[string]string{"PORT": "65535"}, nil, config{"65535", defaultGoogleURL, defaultGoogleAPIKey}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(tt.file, func(key string) string { return tt.env[key] })
			if err != nil || got != tt.want {
				t.Fatalf("config=%+v error=%v want=%+v", got, err, tt.want)
			}
		})
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	for key, values := range map[string][]string{
		"PORT":                 {"0", "65536", "-1", "+80", "abc", "80 80"},
		"GOOGLE_TRANSLATE_URL": {"ftp://example.test", "/relative", "https://", "http://example.test:%", "https://user:private-key@example.test", "https://example.test/#private-key"},
	} {
		for _, value := range values {
			t.Run(key+value, func(t *testing.T) {
				_, err := loadConfig(map[string]string{key: value}, func(string) string { return "" })
				if err == nil || !strings.Contains(err.Error(), key) || strings.Contains(err.Error(), "private-key") {
					t.Fatalf("error=%v", err)
				}
			})
		}
	}
}

func TestLoadRuntimeConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PORT", "9090")
	t.Setenv("GOOGLE_TRANSLATE_URL", "")
	t.Setenv("GOOGLE_TRANSLATE_API_KEY", "")
	cfg, err := loadRuntimeConfig()
	if err != nil || cfg.Port != "9090" {
		t.Fatalf("config=%+v error=%v", cfg, err)
	}
	if _, err := os.Stat(".env"); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GOOGLE_TRANSLATE_URL") != "" || os.Getenv("GOOGLE_TRANSLATE_API_KEY") != "" {
		t.Fatal("configuration mutated process environment")
	}
	if err := os.WriteFile(".env", []byte("PORT=invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORT", "")
	if _, err := loadRuntimeConfig(); err == nil {
		t.Fatal("expected invalid configuration error")
	}
}

func TestLoadRuntimeConfigFileErrors(t *testing.T) {
	for _, exampleError := range []bool{true, false} {
		t.Run(map[bool]string{true: "example", false: "dotenv"}[exampleError], func(t *testing.T) {
			t.Chdir(t.TempDir())
			path, content := ".env.example", ""
			want := "ensure .env"
			if !exampleError {
				path, content = ".env", strings.Repeat("x", 70<<10)
				want = "load .env"
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadRuntimeConfig(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
