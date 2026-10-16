package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppSettings — сохраняемые между запусками настройки приложения.
type AppSettings struct {
	Sequence         string  `json:"sequence"`
	RemoveDuplicates bool    `json:"removeDuplicates"`
	PagePreset       string  `json:"pagePreset"` // имя из PagePresets или "custom"
	CustomWMM        float64 `json:"customWmm"`  // ширина своего формата, мм
	CustomHMM        float64 `json:"customHmm"`  // высота своего формата, мм
	Landscape        bool    `json:"landscape"`
	MarginXMM        float64 `json:"marginXmm"`
	MarginYMM        float64 `json:"marginYmm"`
	Row              int     `json:"row"`
	Col              int     `json:"col"`
	FontFamily       string  `json:"fontFamily"`
	FontSizePt       float64 `json:"fontSizePt"`
	Bold             bool    `json:"bold"`
	Italic           bool    `json:"italic"`
	Color            string  `json:"color"` // имя из colorsByName
	Prefix           string  `json:"prefix"`
	Suffix           string  `json:"suffix"`
	OpenAfterCreate  bool    `json:"openAfterCreate"`
	LastDir          string  `json:"lastDir"`
}

// CustomPageName — пункт «Свой…» в списке размеров.
const CustomPageName = "Свой"

// DefaultSettings — настройки по умолчанию.
func DefaultSettings() AppSettings {
	return AppSettings{
		Sequence:        "1,2,6-9,42-43",
		PagePreset:      "A4",
		CustomWMM:       210,
		CustomHMM:       297,
		MarginXMM:       15,
		MarginYMM:       12,
		Row:             RowBottom,
		Col:             ColCenter,
		FontFamily:      "Helvetica",
		FontSizePt:      12,
		Color:           "black",
		OpenAfterCreate: true,
	}
}

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "blank_num_gen", "settings.json"), nil
}

// LoadSettings читает настройки; при любой ошибке возвращает значения по умолчанию.
func LoadSettings() AppSettings {
	s := DefaultSettings()

	path, err := settingsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings()
	}
	s.normalize()
	return s
}

// SaveSettings сохраняет настройки в %APPDATA%\blank_num_gen\settings.json.
func SaveSettings(s AppSettings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// normalize приводит сохранённые значения к допустимым.
func (s *AppSettings) normalize() {
	d := DefaultSettings()

	if s.Sequence == "" {
		s.Sequence = d.Sequence
	}
	if s.CustomWMM < 20 || s.CustomWMM > 3000 {
		s.CustomWMM = d.CustomWMM
	}
	if s.CustomHMM < 20 || s.CustomHMM > 3000 {
		s.CustomHMM = d.CustomHMM
	}
	s.MarginXMM = clampFloat(s.MarginXMM, 0, 500)
	s.MarginYMM = clampFloat(s.MarginYMM, 0, 500)
	if s.FontSizePt < 4 || s.FontSizePt > 200 {
		s.FontSizePt = d.FontSizePt
	}
	if s.Row < RowTop || s.Row > RowBottom {
		s.Row = d.Row
	}
	if s.Col < ColLeft || s.Col > ColRight {
		s.Col = d.Col
	}
	if _, ok := colorsByName[s.Color]; !ok {
		s.Color = d.Color
	}
	if !containsString(FontPresets, s.FontFamily) {
		s.FontFamily = d.FontFamily
	}
	if !containsString(presetNames(), s.PagePreset) && s.PagePreset != CustomPageName {
		s.PagePreset = d.PagePreset
	}
}

func presetNames() []string {
	names := make([]string, 0, len(PagePresets))
	for _, p := range PagePresets {
		names = append(names, p.Name)
	}
	return names
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// PageSizeMM возвращает размер страницы с учётом выбранного пресета.
func (s AppSettings) PageSizeMM() (w, h float64) {
	for _, p := range PagePresets {
		if p.Name == s.PagePreset {
			return p.W, p.H
		}
	}
	return s.CustomWMM, s.CustomHMM
}
