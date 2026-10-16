package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/opentype"
)

// testFont читает встроенный в проект шрифт для отрисовки тестовых иконок.
func testFont(t *testing.T) *opentype.Font {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "assets", "fonts", "DejaVuSans-Bold.ttf"))
	if err != nil {
		t.Skipf("шрифт недоступен: %v", err)
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		t.Fatalf("шрифт не разобран: %v", err)
	}
	return parsed
}
