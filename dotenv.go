package main

import (
	"bufio"
	_ "embed"
	"errors"
	"io"
	"os"
	"strings"
)

//go:embed .env.example
var embeddedEnvExample string

func ensureDotEnv(path, examplePath string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content, err := os.ReadFile(examplePath)
	if errors.Is(err, os.ErrNotExist) {
		content = []byte(embeddedEnvExample)
	} else if err != nil {
		return err
	}
	if len(content) == 0 {
		return errors.New(".env.example is empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	return errors.Join(writeErr, file.Close())
}

func loadDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseDotEnv(file)
}

func parseDotEnv(reader io.Reader) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(reader)
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()
		if firstLine {
			line = strings.TrimPrefix(line, "\ufeff")
			firstLine = false
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if strings.ContainsRune(key, '\x00') {
			return nil, errors.New("invalid .env key")
		}
		if key == "" || values[key] != "" {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values, scanner.Err()
}
