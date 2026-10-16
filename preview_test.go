package main

import (
	"image"
	"math"
	"strings"
	"testing"
)

func TestRenderPreview(t *testing.T) {
	tests := []struct {
		name string
		opts GenerateOptions
	}{
		{"по умолчанию снизу по центру", baseOptions()},
		{"сверху слева", func() GenerateOptions {
			o := baseOptions()
			o.Row, o.Col = RowTop, ColLeft
			return o
		}()},
		{"по центру справа, кириллица", func() GenerateOptions {
			o := baseOptions()
			o.Row, o.Col = RowMiddle, ColRight
			o.Prefix, o.Suffix = "Стр. ", " —"
			o.FontSizePt = 18
			return o
		}()},
		{"очень крупный номер", func() GenerateOptions {
			o := baseOptions()
			o.FontSizePt = 120
			o.PageWMM, o.PageHMM = 100, 100
			return o
		}()},
		{"альбомная ориентация", func() GenerateOptions {
			o := baseOptions()
			o.Landscape = true
			return o
		}()},
		{"жирный курсив", func() GenerateOptions {
			o := baseOptions()
			o.Bold, o.Italic = true, true
			return o
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := RenderPreview(tt.opts, 42, 300, 330)
			if img == nil {
				t.Fatal("миниатюра не построена")
			}
			if img.Bounds().Dx() != 300 || img.Bounds().Dy() != 330 {
				t.Fatalf("размер миниатюры: %v", img.Bounds())
			}
			assertNotBlank(t, img, tt.name)
		})
	}
}

// assertNotBlank проверяет, что на миниатюре есть что-то кроме фона.
func assertNotBlank(t *testing.T, img image.Image, name string) {
	t.Helper()

	background := img.At(2, 2)
	diff := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r1, g1, b1, _ := img.At(x, y).RGBA()
			r2, g2, b2, _ := background.RGBA()
			// absDiff возвращает разницу в 8-битных единицах по каждому каналу
			if absDiff(r1, r2)+absDiff(g1, g2)+absDiff(b1, b2) > 12 {
				diff++
			}
		}
	}
	if diff < 50 {
		t.Fatalf("%s: миниатюра почти пустая (отличающихся пикселей: %d)", name, diff)
	}
}

func absDiff(a, b uint32) int {
	if a > b {
		return int((a - b) >> 8)
	}
	return int((b - a) >> 8)
}

func TestRenderPreviewInvalid(t *testing.T) {
	opts := baseOptions()

	if img := RenderPreview(opts, 1, 10, 10); img != nil {
		t.Error("ожидалась nil для слишком маленькой области")
	}

	opts.PageWMM = 0
	if img := RenderPreview(opts, 1, 300, 330); img != nil {
		t.Error("ожидалась nil для нулевого размера страницы")
	}
}

func TestRenderPreviewEmbedsFonts(t *testing.T) {
	if len(dejavuSans) == 0 || len(dejavuSansBold) == 0 ||
		len(dejavuSansOblique) == 0 || len(dejavuSansBoldOblique) == 0 {
		t.Fatal("встроенные шрифты не найдены")
	}
	if len(previewFonts()) == 0 {
		t.Fatal("шрифты предпросмотра не разобрались")
	}
}

func TestEmbeddedFontNames(t *testing.T) {
	// имена используются в fpdf как ключи шрифтов
	if DejaVuFamily == "" || strings.ContainsAny(DejaVuFamily, " ") {
		t.Fatalf("некорректное имя шрифта: %q", DejaVuFamily)
	}
}

// inkBounds возвращает границы тёмных пикселей (номера) внутри листа
// вместе с границами самого листа.
func inkBounds(t *testing.T, opts GenerateOptions, number, boxW, boxH int) (page, ink image.Rectangle) {
	t.Helper()

	img := RenderPreview(opts, number, boxW, boxH)
	if img == nil {
		t.Fatal("миниатюра не построена")
	}

	pw, ph := opts.PageSizeMM()
	scale := math.Min(float64(boxW-24)/pw, float64(boxH-24)/ph)
	pageW := int(math.Round(pw * scale))
	pageH := int(math.Round(ph * scale))
	originX := int(math.Round(float64(12) + (float64(boxW-24)-float64(pageW))/2))
	originY := int(math.Round(float64(12) + (float64(boxH-24)-float64(pageH))/2))
	page = image.Rect(originX, originY, originX+pageW, originY+pageH)

	minX, minY, maxX, maxY := math.MaxInt, math.MaxInt, math.MinInt, math.MinInt
	for y := page.Min.Y; y < page.Max.Y; y++ {
		for x := page.Min.X; x < page.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 < 140 && g>>8 < 140 && b>>8 < 140 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if minX > maxX {
		t.Fatal("номер не нарисован")
	}
	return page, image.Rect(minX, minY, maxX, maxY)
}

// TestPreviewNumberPosition проверяет, что номер попадает в нужную ячейку
// сетки 3x3 при любом кегле: раньше при кегле от 20 pt базовая линия
// вычислялась в неверных единицах и номер уезжал наверх.
func TestPreviewNumberPosition(t *testing.T) {
	sizes := []float64{8, 12, 19, 20, 21, 40, 96}

	for _, size := range sizes {
		for row := RowTop; row <= RowBottom; row++ {
			for col := ColLeft; col <= ColRight; col++ {
				opts := baseOptions()
				opts.Row, opts.Col, opts.FontSizePt = row, col, size

				const boxW, boxH = 445, 325
				page, ink := inkBounds(t, opts, 1, boxW, boxH)

				// номер должен помещаться в лист
				inside := page.Inset(-1)
				if !ink.Min.In(inside) || !ink.Max.In(inside) {
					t.Fatalf("%gpt, строка %d, столбец %d: номер %v вышел за лист %v",
						size, row, col, ink, page)
				}

				cx := float64(ink.Min.X+ink.Max.X) / 2
				cy := float64(ink.Min.Y+ink.Max.Y) / 2
				relX := (cx - float64(page.Min.X)) / float64(page.Dx())
				relY := (cy - float64(page.Min.Y)) / float64(page.Dy())

				var wantX float64
				switch col {
				case ColLeft:
					wantX = 0.25
				case ColCenter:
					wantX = 0.5
				default:
					wantX = 0.75
				}
				if math.Abs(relX-wantX) > 0.2 {
					t.Errorf("%gpt, столбец %d: номер по X = %.2f, ожидалось около %.2f (лист %v, номер %v)",
						size, col, relX, wantX, page, ink)
				}

				// по вертикали номер всегда ближе к своей строке, чем к двум другим
				var wantRow int
				switch row {
				case RowTop:
					wantRow = 0
				case RowMiddle:
					wantRow = 1
				default:
					wantRow = 2
				}
				dist := math.Abs(relY - []float64{0.12, 0.5, 0.9}[wantRow])
				for i, target := range []float64{0.12, 0.5, 0.9} {
					if i == wantRow {
						continue
					}
					if math.Abs(relY-target) < dist {
						t.Errorf("%gpt, строка %d: номер по Y = %.2f — ближе к строке %d, чем к своей (лист %v, номер %v)",
							size, row, relY, i, page, ink)
					}
				}
			}
		}
	}
}

// TestPreviewNumberIsReadable проверяет, что номер рисуется настоящими
// глифами, а не заменой-плашкой, при любом кегле.
func TestPreviewNumberIsReadable(t *testing.T) {
	for _, size := range []float64{8, 12, 19, 20, 21, 40} {
		opts := baseOptions()
		opts.FontSizePt = size
		opts.Prefix, opts.Suffix = "Стр. ", " —"

		page, ink := inkBounds(t, opts, 42, 445, 325)

		// высота номера должна быть читаемой, а не однопиксельной линией
		if ink.Dy() < 8 {
			t.Errorf("%gpt: высота номера %d px — нечитаемо (номер %v, лист %v)", size, ink.Dy(), ink, page)
		}
		// и не вылезать за лист
		if ink.Dx() >= page.Dx() {
			t.Errorf("%gpt: номер шире листа (%v против %v)", size, ink, page)
		}
	}
}

// TestPreviewStaysInsidePage проверяет, что номер и направляющая никогда не
// выходят за пределы листа, в том числе при нулевых отступах: раньше при
// отступе снизу 0 мм настоящая строка упиралась в край страницы и тонкая
// линия уезжала на пиксель за границу миниатюры.
func TestPreviewStaysInsidePage(t *testing.T) {
	for _, margin := range []float64{0, 1, 5, 15} {
		for row := RowTop; row <= RowBottom; row++ {
			for col := ColLeft; col <= ColRight; col++ {
				for _, size := range []float64{6, 8, 12, 19, 20, 33, 60} {
					opts := baseOptions()
					opts.Row, opts.Col, opts.FontSizePt = row, col, size
					opts.MarginXMM, opts.MarginYMM = margin, margin

					const boxW, boxH = 445, 325
					img := RenderPreview(opts, 1, boxW, boxH)
					if img == nil {
						t.Fatalf("отступ %g, строка %d, столбец %d, %gpt: миниатюра не построена",
							margin, row, col, size)
					}

					pw, ph := opts.PageSizeMM()
					scale := math.Min(float64(boxW-24)/pw, float64(boxH-24)/ph)
					pageW := int(math.Round(pw * scale))
					pageH := int(math.Round(ph * scale))
					originX := int(math.Round(float64(12) + (float64(boxW-24)-float64(pageW))/2))
					originY := int(math.Round(float64(12) + (float64(boxH-24)-float64(pageH))/2))
					page := image.Rect(originX, originY, originX+pageW, originY+pageH)

					// вокруг листа допускается только тень шириной в несколько пикселей
					halo := page.Inset(-5)
					for y := halo.Min.Y; y < halo.Max.Y; y++ {
						if y < 0 || y >= boxH {
							continue
						}
						for x := halo.Min.X; x < halo.Max.X; x++ {
							if x < 0 || x >= boxW || image.Pt(x, y).In(page) {
								continue
							}
							r, g, b, _ := img.At(x, y).RGBA()
							// фон миниатюры — 0xE6E8EC, тень — 0xC8CBD2
							if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
								t.Fatalf("отступ %g, строка %d, столбец %d, %gpt: пиксель (%d,%d)=%d,%d,%d вне листа %v",
									margin, row, col, size, x, y, r>>8, g>>8, b>>8, page)
							}
						}
					}
				}
			}
		}
	}
}
