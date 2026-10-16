package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func baseOptions() GenerateOptions {
	return GenerateOptions{
		Numbers:    []int{1, 2, 3},
		PageWMM:    210,
		PageHMM:    297,
		MarginXMM:  15,
		MarginYMM:  12,
		Row:        RowBottom,
		Col:        ColCenter,
		FontFamily: "Helvetica",
		FontSizePt: 12,
		Color:      ColorChoices[0].RGB,
	}
}

func TestGeneratePDFPages(t *testing.T) {
	numbers := []int{1, 2, 6, 7, 8, 9, 42, 43}
	opts := baseOptions()
	opts.Numbers = numbers

	var buf bytes.Buffer
	if err := GeneratePDF(&buf, opts); err != nil {
		t.Fatalf("GeneratePDF: %v", err)
	}

	data := buf.Bytes()
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("файл не начинается с %PDF-")
	}
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("в файле нет маркера EOF")
	}

	// fpdf не сжимает словари объектов, поэтому число страниц видно по "/Type /Page".
	got := bytes.Count(data, []byte("/Type /Page\n"))
	if got != len(numbers) {
		t.Fatalf("страниц в файле: %d, ожидалось %d", got, len(numbers))
	}

	// каждая страница должна ссылаться на содержимое
	if n := bytes.Count(data, []byte("/Contents")); n != len(numbers) {
		t.Fatalf("ссылок /Contents: %d, ожидалось %d", n, len(numbers))
	}
}

func TestGeneratePDFSingleNumber(t *testing.T) {
	opts := baseOptions()
	opts.Numbers = []int{42}

	var buf bytes.Buffer
	if err := GeneratePDF(&buf, opts); err != nil {
		t.Fatalf("GeneratePDF: %v", err)
	}
	if n := bytes.Count(buf.Bytes(), []byte("/Type /Page\n")); n != 1 {
		t.Fatalf("страниц: %d, ожидалась 1", n)
	}
}

func TestGeneratePDFValidation(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*GenerateOptions)
	}{
		{"без номеров", func(o *GenerateOptions) { o.Numbers = nil }},
		{"нулевой размер", func(o *GenerateOptions) { o.PageWMM = 0 }},
		{"слишком маленький лист", func(o *GenerateOptions) { o.PageWMM = 5 }},
		{"нулевой шрифт", func(o *GenerateOptions) { o.FontSizePt = 0 }},
		{"плохой отступ", func(o *GenerateOptions) { o.MarginXMM = 200 }},
		{"плохая строка", func(o *GenerateOptions) { o.Row = 7 }},
		{"плохой столбец", func(o *GenerateOptions) { o.Col = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := baseOptions()
			tt.mut(&opts)
			var buf bytes.Buffer
			if err := GeneratePDF(&buf, opts); err == nil {
				t.Fatal("ожидалась ошибка")
			}
		})
	}
}

func TestGeneratePDFEmbedsCyrillicFont(t *testing.T) {
	opts := baseOptions()
	opts.Numbers = []int{1}
	opts.Prefix = "Стр. "
	opts.Suffix = " —"

	var buf bytes.Buffer
	if err := GeneratePDF(&buf, opts); err != nil {
		t.Fatalf("GeneratePDF: %v", err)
	}
	// встроенный шрифт объявляется в объекте шрифтов
	if !bytes.Contains(buf.Bytes(), []byte("/BaseFont")) {
		t.Fatal("в PDF нет встроенного шрифта")
	}
}

func TestPageSizeSwap(t *testing.T) {
	opts := baseOptions()
	if w, h := opts.PageSizeMM(); w != 210 || h != 297 {
		t.Fatalf("книжная: %v x %v", w, h)
	}
	opts.Landscape = true
	if w, h := opts.PageSizeMM(); w != 297 || h != 210 {
		t.Fatalf("альбомная: %v x %v", w, h)
	}
}

func TestRowY(t *testing.T) {
	opts := baseOptions() // 210x297, поля 12 мм сверху и снизу
	lh := opts.LineHeightMM()
	ph := 297.0

	opts.Row = RowTop
	if got := opts.RowY(lh, ph); got != 12 {
		t.Errorf("сверху: Y = %.2f, ожидалось 12", got)
	}

	opts.Row = RowMiddle
	want := (ph - lh) / 2
	if got := opts.RowY(lh, ph); got != want {
		t.Errorf("по центру: Y = %.2f, ожидалось %.2f", got, want)
	}

	opts.Row = RowBottom
	want = ph - 12 - lh
	if got := opts.RowY(lh, ph); got != want {
		t.Errorf("снизу: Y = %.2f, ожидалось %.2f", got, want)
	}
}

func TestLabelFor(t *testing.T) {
	if got := LabelFor(42, "— ", " —"); got != "— 42 —" {
		t.Fatalf("получено %q", got)
	}
	if got := LabelFor(7, "", ""); got != "7" {
		t.Fatalf("получено %q", got)
	}
}

func TestCLIGeneratesPDF(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "pages.pdf")

	var buf bytes.Buffer
	args := []string{
		"-o", out,
		"-s", "1,2,6-9",
		"-pos", "top-right",
		"-size", "Letter",
		"-landscape",
		"-margin-x", "20",
		"-margin-y", "25",
		"-font", "Times",
		"-pt", "14",
		"-bold",
		"-color", "blue",
		"-prefix", "№ ",
	}
	if err := runCLI(args, &buf); err != nil {
		t.Fatalf("runCLI: %v", err)
	}
	if !strings.Contains(buf.String(), "Создано страниц: 6") {
		t.Fatalf("неожиданный вывод: %q", buf.String())
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("файл не создан: %v", err)
	}
	if n := bytes.Count(data, []byte("/Type /Page\n")); n != 6 {
		t.Fatalf("страниц: %d, ожидалось 6", n)
	}
}

func TestParsePageSize(t *testing.T) {
	tests := []struct {
		in       string
		wantW    float64
		wantH    float64
		wantFail bool
	}{
		{in: "A4", wantW: 210, wantH: 297},
		{in: "letter", wantW: 215.9, wantH: 279.4},
		{in: "148x210", wantW: 148, wantH: 210},
		{in: "148X210", wantW: 148, wantH: 210},
		{in: "200*100", wantW: 200, wantH: 100},
		{in: "мусор", wantFail: true},
	}

	for _, tt := range tests {
		w, h, err := parsePageSize(tt.in)
		if tt.wantFail {
			if err == nil {
				t.Errorf("parsePageSize(%q): ожидалась ошибка", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePageSize(%q): %v", tt.in, err)
			continue
		}
		if w != tt.wantW || h != tt.wantH {
			t.Errorf("parsePageSize(%q) = %v x %v, ожидалось %v x %v", tt.in, w, h, tt.wantW, tt.wantH)
		}
	}
}

func TestParsePosition(t *testing.T) {
	tests := []struct {
		in       string
		wantRow  int
		wantCol  int
		wantFail bool
	}{
		{"bottom-center", RowBottom, ColCenter, false},
		{"top-left", RowTop, ColLeft, false},
		{"снизу-справа", RowBottom, ColRight, false},
		{"middle", RowMiddle, ColCenter, false},
		{"right", RowBottom, ColRight, false},
		{"nonsense", 0, 0, true},
	}

	for _, tt := range tests {
		row, col, err := parsePosition(tt.in)
		if tt.wantFail {
			if err == nil {
				t.Errorf("parsePosition(%q): ожидалась ошибка", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePosition(%q): %v", tt.in, err)
			continue
		}
		if row != tt.wantRow || col != tt.wantCol {
			t.Errorf("parsePosition(%q) = %d,%d, ожидалось %d,%d", tt.in, row, col, tt.wantRow, tt.wantCol)
		}
	}
}

func TestRunCLIMissingOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := runCLI([]string{"-s", "1"}, &buf); err == nil {
		t.Fatal("ожидалась ошибка: не указан -o")
	}
}

// pdfTextBaselineMM достаёт из незашифрованного PDF координату базовой линии
// текста (в мм от верхнего края страницы), которую записал fpdf.
func pdfTextBaselineMM(t *testing.T, data []byte, opts GenerateOptions) float64 {
	t.Helper()

	// fpdf пишет текст так: BT x y Td (...)Tj ET
	const marker = ")Tj ET"
	i := bytes.Index(data, []byte(marker))
	if i < 0 {
		t.Fatal("в PDF не найден текстовый оператор Tj")
	}
	// координаты стоят перед "Td": "BT x y Td"
	head := data[:i]
	j := bytes.LastIndex(head, []byte("BT "))
	if j < 0 {
		t.Fatal("не найден блок BT")
	}
	rest := head[j+3:]
	k := bytes.Index(rest, []byte("Td"))
	if k < 0 {
		t.Fatalf("не найден оператор Td: %q", head[j:i])
	}
	fields := bytes.Fields(rest[:k])
	if len(fields) < 2 {
		t.Fatalf("не разобраны координаты текста: %q", head[j:i])
	}
	y, err := strconv.ParseFloat(string(fields[1]), 64)
	if err != nil {
		t.Fatalf("координата Y %q: %v", fields[1], err)
	}
	// в PDF точка отсчёта — нижний край страницы, нам нужна высота над ним
	_, pageH := opts.PageSizeMM()
	return (pageH*72/25.4 - y) * 25.4 / 72 // пункты → миллиметры от верхнего края
}

// TestPDFBaselineMatchesMargins проверяет, что в готовом PDF отступы от края
// до края цифр сверху и снизу равны заданному значению: fpdf размещает текст
// в ячейке по своей формуле, поэтому верх ячейки приходится считать самим.
func TestPDFBaselineMatchesMargins(t *testing.T) {
	fonts := []string{"Helvetica", "Times", "Courier", DejaVuFamily}
	sizes := []float64{8, 12, 22, 60}
	pages := []PagePreset{{Name: "A4", W: 210, H: 297}, {Name: "A3", W: 297, H: 420}, {Name: "Letter", W: 215.9, H: 279.4}}
	rows := []int{RowTop, RowMiddle, RowBottom}

	for _, font := range fonts {
		for _, size := range sizes {
			for _, page := range pages {
				for _, row := range rows {
					opts := baseOptions()
					opts.Numbers = []int{42}
					opts.PageWMM, opts.PageHMM = page.W, page.H
					opts.FontFamily, opts.FontSizePt = font, size
					opts.MarginXMM, opts.MarginYMM = 15, 12
					opts.Row, opts.Prefix = row, "A" // ASCII — без встроенного шрифта
					if font == DejaVuFamily {
						opts.Prefix = "А" // кириллица, чтобы шрифт реально подхватился
					}

					var buf bytes.Buffer
					if err := generatePDF(&buf, opts, false); err != nil {
						t.Fatalf("%s %gpt %s строка %d: %v", font, size, page.Name, row, err)
					}

					got := pdfTextBaselineMM(t, buf.Bytes(), opts)
					want := opts.BaselineMM()
					if math.Abs(got-want) > 0.2 {
						t.Errorf("%s %gpt %s строка %d: базовая линия %.2f мм, ожидалось %.2f мм",
							font, size, page.Name, row, got, want)
					}
				}
			}
		}
	}
}

// TestPDFMarginsAreSymmetric проверяет, что одинаковый отступ сверху и снизу
// даёт одинаковый видимый зазор до края листа.
func TestPDFMarginsAreSymmetric(t *testing.T) {
	for _, font := range []string{"Helvetica", "Times", DejaVuFamily} {
		for _, size := range []float64{8, 12, 22, 60} {
			const margin = 12.0

			top := baseOptions()
			top.Numbers = []int{1}
			top.FontFamily, top.FontSizePt = font, size
			top.MarginYMM, top.Row = margin, RowTop

			bottom := top
			bottom.Row = RowBottom

			cap, descent := fontVerticalMetrics(font)
			fs := size * 25.4 / 72
			_, ph := top.PageSizeMM()

			topInk := top.BaselineMM() - cap*fs
			bottomInk := ph - (bottom.BaselineMM() + descent*fs)

			if math.Abs(topInk-margin) > 0.001 || math.Abs(bottomInk-margin) > 0.001 {
				t.Errorf("%s %gpt: сверху %.3f мм, снизу %.3f мм, ожидалось по %.1f мм",
					font, size, topInk, bottomInk, margin)
			}
		}
	}
}
