package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	text := "\ufeffPORT=9090\r\n# 註解\r\n\ninvalid line\n =ignored\nKEY='value=with=equals'\nDOUBLE=\"value\"\nKEY=second\nEMPTY=\nEMPTY=filled\n"
	got, err := parseDotEnv(strings.NewReader(text))
	want := map[string]string{"PORT": "9090", "KEY": "value=with=equals", "DOUBLE": "value", "EMPTY": "filled"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("values=%v error=%v", got, err)
	}
	for name, reader := range map[string]*strings.Reader{
		"invalid key": strings.NewReader("BAD\x00KEY=value\n"),
		"long line":   strings.NewReader(strings.Repeat("x", 70<<10)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseDotEnv(reader); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
	if _, err := parseDotEnv(errorReader{}); err == nil {
		t.Fatal("expected reader error")
	}
}

func TestLoadDotEnv(t *testing.T) {
	t.Setenv("DOTENV_EXISTING", "from-env")
	t.Setenv("DOTENV_VALUE", "")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DOTENV_VALUE=from-file\nDOTENV_EXISTING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := loadDotEnv(path)
	if err != nil || values["DOTENV_VALUE"] != "from-file" || values["DOTENV_EXISTING"] != "from-file" {
		t.Fatalf("values=%v error=%v", values, err)
	}
	if os.Getenv("DOTENV_EXISTING") != "from-env" || os.Getenv("DOTENV_VALUE") != "" {
		t.Fatal("loadDotEnv modified environment")
	}
	if values, err := loadDotEnv(path + ".missing"); err != nil || len(values) != 0 {
		t.Fatalf("missing values=%v error=%v", values, err)
	}
	if _, err := loadDotEnv(path + "\x00"); err == nil {
		t.Fatal("expected open error")
	}
}

func TestEnsureDotEnv(t *testing.T) {
	for _, mode := range []string{"external", "existing", "embedded", "concurrent"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path, example := filepath.Join(dir, ".env"), filepath.Join(dir, ".env.example")
			want := []byte("PORT=9090\n")
			if mode == "embedded" {
				want = []byte(embeddedEnvExample)
			} else if err := os.WriteFile(example, want, 0o600); err != nil {
				t.Fatal(err)
			}
			if mode == "existing" {
				want = []byte("PORT=7070\n")
				if err := os.WriteFile(path, want, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var wg sync.WaitGroup
			workers := 1
			if mode == "concurrent" {
				workers = 8
			}
			for range workers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := ensureDotEnv(path, example); err != nil {
						t.Errorf("ensureDotEnv: %v", err)
					}
				}()
			}
			wg.Wait()
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("content=%q error=%v want=%q", got, err, want)
			}
		})
	}
}

func TestEnsureDotEnvFailures(t *testing.T) {
	dir := t.TempDir()
	example := filepath.Join(dir, ".env.example")
	if err := os.WriteFile(example, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureDotEnv(filepath.Join(dir, ".env"), example); err == nil {
		t.Fatal("expected empty example error")
	}
	if err := ensureDotEnv(filepath.Join(dir, "missing", ".env"), example+".missing"); err == nil {
		t.Fatal("expected create error")
	}
	if err := ensureDotEnv(filepath.Join(dir, ".env"), example+"\x00"); err == nil {
		t.Fatal("expected example read error")
	}
	if err := ensureDotEnv(example+"\x00", example); err == nil {
		t.Fatal("expected stat error")
	}
}
