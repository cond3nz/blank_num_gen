package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode"

	"github.com/go-pdf/fpdf"
)

// Row и Col задают положение номера на странице (сетка 3x3).
const (
	RowTop = iota
	RowMiddle
	RowBottom
)

const (
	ColLeft = iota
	ColCenter
	ColRight
)

// RGB — цвет текста в PDF.
type RGB struct{ R, G, B int }

// ColorChoice — цвет номера, доступный в GUI.
type ColorChoice struct {
	Name  string
	Label string
	RGB   RGB
}

// ColorChoices — палитра номера.
var ColorChoices = []ColorChoice{
	{Name: "black", Label: "Чёрный", RGB: RGB{0, 0, 0}},
	{Name: "darkgray", Label: "Тёмно-серый", RGB: RGB{48, 48, 48}},
	{Name: "gray", Label: "Серый", RGB: RGB{110, 110, 110}},
	{Name: "lightgray", Label: "Светло-серый", RGB: RGB{170, 170, 170}},
	{Name: "blue", Label: "Синий", RGB: RGB{0, 0, 153}},
	{Name: "red", Label: "Красный", RGB: RGB{178, 0, 0}},
	{Name: "green", Label: "Зелёный", RGB: RGB{0, 97, 0}},
	{Name: "orange", Label: "Оранжевый", RGB: RGB{200, 90, 0}},
}

var colorsByName = func() map[string]RGB {
	m := make(map[string]RGB, len(ColorChoices))
	for _, c := range ColorChoices {
		m[c.Name] = c.RGB
	}
	return m
}()

// ColorNames возвращает имена цветов в порядке отображения.
func ColorNames() []string {
	names := make([]string, 0, len(ColorChoices))
	for _, c := range ColorChoices {
		names = append(names, c.Name)
	}
	return names
}

// ColorLabels возвращает подписи цветов в порядке отображения.
func ColorLabels() []string {
	labels := make([]string, 0, len(ColorChoices))
	for _, c := range ColorChoices {
		labels = append(labels, c.Label)
	}
	return labels
}

// PagePreset — именованный размер страницы в миллиметрах (книжная ориентация).
type PagePreset struct {
	Name string
	W, H float64
}

// PagePresets — размеры, доступные в GUI.
var PagePresets = []PagePreset{
	{Name: "A4", W: 210, H: 297},
	{Name: "A3", W: 297, H: 420},
	{Name: "Letter", W: 215.9, H: 279.4},
	{Name: "Legal", W: 215.9, H: 355.6},
	{Name: "A5", W: 148, H: 210},
}

// FontPresets — гарнитуры, доступные в GUI.
var FontPresets = []string{"Helvetica", "Times", "Courier"}

// GenerateOptions — всё, что нужно для создания PDF.
type GenerateOptions struct {
	Numbers []int // номера страниц; один номер — одна страница

	PageWMM   float64 // ширина страницы (книжная ориентация), мм
	PageHMM   float64 // высота страницы (книжная ориентация), мм
	Landscape bool

	MarginXMM float64 // отступ слева и справа, мм
	MarginYMM float64 // отступ сверху и снизу, мм

	Row, Col int // положение номера на странице

	FontFamily string
	FontSizePt float64
	Bold       bool
	Italic     bool
	Color      RGB

	Prefix string // например "— " или "Стр. "
	Suffix string

	Title string
}

// DejaVuFamily — семейство шрифтов, встроенных в приложение и используемых
// автоматически, если в префиксе/суффиксе есть не-ASCII символы.
const DejaVuFamily = "DejaVuSans"

// ErrNoNumbers возвращается, если список номеров пуст.
var ErrNoNumbers = errors.New("не задано ни одного номера страницы")

// PageSizeMM возвращает итоговый размер листа с учётом ориентации.
func (o GenerateOptions) PageSizeMM() (w, h float64) {
	if o.Landscape {
		return o.PageHMM, o.PageWMM
	}
	return o.PageWMM, o.PageHMM
}

// LabelFor возвращает текст, который печатается для номера n.
func LabelFor(n int, prefix, suffix string) string {
	return prefix + strconv.Itoa(n) + suffix
}

// LineHeightMM — высота строки с номером (в мм).
func (o GenerateOptions) LineHeightMM() float64 {
	return o.FontSizePt * 25.4 / 72 * 1.2
}

// Validate проверяет параметры и возвращает понятную пользователю ошибку.
func (o GenerateOptions) Validate() error {
	if len(o.Numbers) == 0 {
		return ErrNoNumbers
	}
	if o.PageWMM <= 0 || o.PageHMM <= 0 {
		return fmt.Errorf("неверный размер страницы: %.1f x %.1f мм", o.PageWMM, o.PageHMM)
	}
	if o.PageWMM < 20 || o.PageHMM < 20 {
		return fmt.Errorf("слишком маленький размер страницы: %.1f x %.1f мм", o.PageWMM, o.PageHMM)
	}
	if o.FontSizePt <= 0 {
		return fmt.Errorf("неверный размер шрифта: %.1f pt", o.FontSizePt)
	}
	if o.Row < RowTop || o.Row > RowBottom || o.Col < ColLeft || o.Col > ColRight {
		return errors.New("неверное положение номера на странице")
	}
	pw, ph := o.PageSizeMM()
	lh := o.LineHeightMM()
	if o.MarginXMM < 0 || o.MarginYMM < 0 {
		return errors.New("отступы не могут быть отрицательными")
	}
	if 2*o.MarginXMM >= pw {
		return fmt.Errorf("боковой отступ %.1f мм слишком велик для ширины %.1f мм", o.MarginXMM, pw)
	}
	if o.MarginYMM+lh >= ph {
		return fmt.Errorf("вертикальный отступ %.1f мм слишком велик для высоты %.1f мм", o.MarginYMM, ph)
	}
	return nil
}

// needsEmbeddedFont сообщает, требуется ли встроенный шрифт (для кириллицы).
func (o GenerateOptions) needsEmbeddedFont() bool {
	return containsNonASCII(o.Prefix) || containsNonASCII(o.Suffix)
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func fontStyleCode(bold, italic bool) string {
	switch {
	case bold && italic:
		return "BI"
	case bold:
		return "B"
	case italic:
		return "I"
	default:
		return ""
	}
}

// alignmentFor возвращает выравнивание строки по горизонтали.
func alignmentFor(col int) string {
	switch col {
	case ColCenter:
		return "C"
	case ColRight:
		return "R"
	default:
		return "L"
	}
}

// GeneratePDF создаёт PDF с пустыми страницами, на каждой из которых
// в заданной позиции напечатан соответствующий номер.
func GeneratePDF(w io.Writer, opts GenerateOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	orientation := "P"
	if opts.Landscape {
		orientation = "L"
	}

	pdf := fpdf.New(orientation, "mm", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetCompression(true)
	if opts.Title != "" {
		pdf.SetTitle(opts.Title, true)
	}

	family := opts.FontFamily
	if family == "" {
		family = "Helvetica"
	}
	if opts.needsEmbeddedFont() {
		if err := registerEmbeddedFonts(pdf); err != nil {
			return err
		}
		family = DejaVuFamily
	}
	style := fontStyleCode(opts.Bold, opts.Italic)

	pw, ph := opts.PageSizeMM()
	lh := opts.LineHeightMM()
	boxW := pw - 2*opts.MarginXMM
	align := alignmentFor(opts.Col)

	for _, n := range opts.Numbers {
		pdf.AddPageFormat(orientation, fpdf.SizeType{Wd: opts.PageWMM, Ht: opts.PageHMM})
		pdf.SetFont(family, style, opts.FontSizePt)
		pdf.SetTextColor(opts.Color.R, opts.Color.G, opts.Color.B)

		pdf.SetXY(opts.MarginXMM, opts.RowY(lh, ph))
		pdf.CellFormat(boxW, lh, LabelFor(n, opts.Prefix, opts.Suffix), "", 0, align, false, 0, "")
	}

	if err := pdf.Error(); err != nil {
		return err
	}
	return pdf.Output(w)
}

// RowY возвращает координату Y (в мм) для строки в заданной ячейке сетки 3x3.
func (o GenerateOptions) RowY(lineHeight, pageHeight float64) float64 {
	switch o.Row {
	case RowMiddle:
		return (pageHeight - lineHeight) / 2
	case RowBottom:
		return pageHeight - o.MarginYMM - lineHeight
	default:
		return o.MarginYMM
	}
}

// registerEmbeddedFonts добавляет встроенные шрифты DejaVu (для кириллицы).
func registerEmbeddedFonts(pdf *fpdf.Fpdf) error {
	fonts := []struct {
		style string
		data  []byte
	}{
		{"", dejavuSans},
		{"B", dejavuSansBold},
		{"I", dejavuSansOblique},
		{"BI", dejavuSansBoldOblique},
	}
	for _, f := range fonts {
		if len(f.data) == 0 {
			return errors.New("встроенный шрифт не найден")
		}
		pdf.AddUTF8FontFromBytes(DejaVuFamily, f.style, f.data)
	}
	return nil
}
