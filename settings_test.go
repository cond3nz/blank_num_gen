package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// settingsFile возвращает путь к файлу настроек внутри временного каталога.
func settingsFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "blank_num_gen", "settings.json")
}

func TestSettingsRoundTrip(t *testing.T) {
	path := settingsFile(t)

	want := DefaultSettings()
	want.Sequence = "3-10,42"
	want.RemoveDuplicates = true
	want.PagePreset = "A3"
	want.Landscape = true
	want.MarginXMM = 20
	want.MarginYMM = 25
	want.Row = RowTop
	want.Col = ColLeft
	want.FontFamily = "Courier"
	want.FontSizePt = 18
	want.Bold = true
	want.Italic = true
	want.Color = "blue"
	want.Prefix = "Стр. "
	want.Suffix = " —"
	want.OpenAfterCreate = false
	want.LastDir = filepath.Join("some", "dir")

	if err := SaveSettingsTo(path, want); err != nil {
		t.Fatalf("SaveSettingsTo: %v", err)
	}

	got := LoadSettingsFrom(path)
	if got != want {
		t.Fatalf("настройки не совпали:\nполучено  %+v\nожидалось %+v", got, want)
	}
}

func TestLoadSettingsWithoutFile(t *testing.T) {
	if got := LoadSettingsFrom(settingsFile(t)); got != DefaultSettings() {
		t.Fatalf("ожидались настройки по умолчанию, получено %+v", got)
	}
}

func TestLoadSettingsBrokenFile(t *testing.T) {
	path := settingsFile(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{это не json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := LoadSettingsFrom(path); got != DefaultSettings() {
		t.Fatalf("битый файл: ожидались значения по умолчанию, получено %+v", got)
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := AppSettings{
		CustomWMM:  1,
		CustomHMM:  -3,
		MarginXMM:  -10,
		MarginYMM:  5000,
		FontSizePt: 9999,
		Row:        42,
		Col:        -3,
		Color:      "нет такого",
		FontFamily: "Comic Sans",
		PagePreset: "неизвестный",
	}
	s.normalize()

	d := DefaultSettings()
	if s.Sequence != d.Sequence || s.CustomWMM != d.CustomWMM || s.CustomHMM != d.CustomHMM ||
		s.FontSizePt != d.FontSizePt || s.Row != d.Row || s.Col != d.Col ||
		s.Color != d.Color || s.FontFamily != d.FontFamily || s.PagePreset != d.PagePreset {
		t.Fatalf("нормализация не сработала: %+v", s)
	}
	if s.MarginXMM < 0 || s.MarginYMM < 0 {
		t.Fatalf("отступы должны быть неотрицательными: %+v", s)
	}
}

func TestSettingsPageSize(t *testing.T) {
	s := DefaultSettings()
	if w, h := s.PageSizeMM(); w != 210 || h != 297 {
		t.Fatalf("A4: %v x %v", w, h)
	}

	s.PagePreset = "A3"
	if w, h := s.PageSizeMM(); w != 297 || h != 420 {
		t.Fatalf("A3: %v x %v", w, h)
	}

	s.PagePreset = CustomPageName
	s.CustomWMM, s.CustomHMM = 148, 210
	if w, h := s.PageSizeMM(); w != 148 || h != 210 {
		t.Fatalf("свой размер: %v x %v", w, h)
	}
}

func TestSaveSettingsJSONIsReadable(t *testing.T) {
	path := settingsFile(t)

	if err := SaveSettingsTo(path, DefaultSettings()); err != nil {
		t.Fatalf("SaveSettingsTo: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("файл настроек не найден: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("файл настроек не является корректным JSON: %v", err)
	}
	if _, ok := raw["sequence"]; !ok {
		t.Fatal("в файле нет поля sequence")
	}
}

func TestSaveSettingsOverwrites(t *testing.T) {
	path := settingsFile(t)

	first := DefaultSettings()
	first.Sequence = "1-3"
	if err := SaveSettingsTo(path, first); err != nil {
		t.Fatal(err)
	}

	second := DefaultSettings()
	second.Sequence = "10-20"
	if err := SaveSettingsTo(path, second); err != nil {
		t.Fatalf("повторное сохранение: %v", err)
	}

	if got := LoadSettingsFrom(path); got.Sequence != second.Sequence {
		t.Fatalf("ожидалась последняя запись %q, получено %q", second.Sequence, got.Sequence)
	}
}

func TestSettingsPathInUserConfigDir(t *testing.T) {
	path, err := settingsPath()
	if err != nil {
		t.Skipf("каталог настроек недоступен: %v", err)
	}
	if filepath.Base(filepath.Dir(path)) != "blank_num_gen" {
		t.Fatalf("неожиданный путь настроек: %s", path)
	}
	if filepath.Base(path) != "settings.json" {
		t.Fatalf("неожиданное имя файла настроек: %s", path)
	}
}
