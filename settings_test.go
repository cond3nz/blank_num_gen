package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))

	want := DefaultSettings()
	want.Sequence = "3-10,42"
	want.RemoveDuplicates = true
	want.PagePreset = "Legal"
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
	want.LastDir = "/tmp"

	if err := SaveSettings(want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got := LoadSettings()
	if got != want {
		t.Fatalf("настройки не совпали:\nполучено  %+v\nожидалось %+v", got, want)
	}
}

func TestLoadSettingsWithoutFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))

	if got := LoadSettings(); got != DefaultSettings() {
		t.Fatalf("ожидались настройки по умолчанию, получено %+v", got)
	}
}

func TestLoadSettingsBrokenFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))

	path := filepath.Join(dir, "blank_num_gen", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{это не json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := LoadSettings(); got != DefaultSettings() {
		t.Fatalf("битый файл: ожидались значения по умолчанию, получено %+v", got)
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := AppSettings{
		Sequence:   "",
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
	s.PagePreset = CustomPageName
	s.CustomWMM, s.CustomHMM = 148, 210
	if w, h := s.PageSizeMM(); w != 148 || h != 210 {
		t.Fatalf("свой размер: %v x %v", w, h)
	}
}

func TestSaveSettingsJSONIsReadable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))

	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	path := filepath.Join(dir, "blank_num_gen", "settings.json")
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
