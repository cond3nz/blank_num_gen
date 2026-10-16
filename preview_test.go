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

// TestPreviewBaselineIsFixed проверяет, что положение цифры в миниатюре не
// зависит от кегля: она всегда стоит на одном и том же месте относительно
// края. Раньше цифра центрировалась по строке, поэтому с увеличением кегля
// расстояние до настоящей линии росло.
func TestPreviewBaselineIsFixed(t *testing.T) {
	const boxW, boxH = 445, 325

	for _, margin := range []float64{0, 5, 15} {
		for row := RowTop; row <= RowBottom; row++ {
			for col := ColLeft; col <= ColRight; col++ {
				var previousGap = math.NaN()

				for _, size := range []float64{6, 8, 12, 19, 20, 33, 60, 120} {
					opts := baseOptions()
					opts.Row, opts.Col, opts.FontSizePt = row, col, size
					opts.MarginXMM, opts.MarginYMM = margin, margin

					page, ink := inkBounds(t, opts, 1, boxW, boxH)

					pw, ph := opts.PageSizeMM()
					scale := math.Min(float64(boxW-24)/pw, float64(boxH-24)/ph)
					fontPx := math.Max(size*25.4/72*scale, minPreviewFontPx)

					// независимо пересчитываем ожидаемую базовую линию
					face := newPreviewFace(opts.Bold, opts.Italic, fontPx)
					if face == nil {
						t.Fatalf("%gpt: не удалось получить метрики шрифта", size)
					}
					descent := float64(face.Metrics().Descent) / 64
					face.Close()

					expected := previewBaseline(opts, float64(page.Min.Y), scale, fontPx)
					minB := float64(page.Min.Y) + 0.8*fontPx + 1
					maxB := float64(page.Max.Y) - descent*1.05 - 1
					if maxB < minB {
						expected = clampFloat(expected, float64(page.Min.Y)+1, float64(page.Max.Y)-2)
					} else {
						expected = clampFloat(expected, minB, maxB)
					}

					// низ глифа на 1-2 px выше базовой линии из-за сглаживания
					gap := float64(ink.Max.Y) - expected
					if math.Abs(gap) > 4 {
						t.Errorf("отступ %g, строка %d, столбец %d, %gpt: низ цифры на %.1f px, ожидалось около %.1f px (лист %v, цифра %v)",
							margin, row, col, size, float64(ink.Max.Y), expected, page, ink)
					}
					if !math.IsNaN(previousGap) && math.Abs(gap-previousGap) > 2 {
						t.Errorf("отступ %g, строка %d, столбец %d: смещение изменилось с %.1f на %.1f px при смене кегля",
							margin, row, col, previousGap, gap)
					}
					previousGap = gap
				}
			}
		}
	}
}

// TestPreviewHasNoHelperLine проверяет, что в миниатюре нет отдельной
// вспомогательной линии: при нулевом отступе сверху она пересекала цифру.
func TestPreviewHasNoHelperLine(t *testing.T) {
	opts := baseOptions()
	opts.Row = RowTop
	opts.MarginYMM = 0
	opts.FontSizePt = 12

	const boxW, boxH = 445, 325
	img := RenderPreview(opts, 1, boxW, boxH)
	if img == nil {
		t.Fatal("миниатюра не построена")
	}

	pw, ph := opts.PageSizeMM()
	scale := math.Min(float64(boxW-24)/pw, float64(boxH-24)/ph)
	pageW := int(math.Round(pw * scale))
	pageH := int(math.Round(ph * scale))
	originX := int(math.Round(float64(12) + (float64(boxW-24)-float64(pageW))/2))
	originY := int(math.Round(float64(12) + (float64(boxH-24)-float64(pageH))/2))

	// верхние 40 px листа: если там только цифра, тёмных строк будет немного
	darkRows := 0
	for y := originY; y < originY+40; y++ {
		for x := originX; x < originX+pageW; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 < 140 && g>>8 < 140 && b>>8 < 140 {
				darkRows++
				break
			}
		}
	}
	// цифра «1» занимает около 20 строк; линия на всю ширину дала бы больше
	if darkRows > 26 {
		t.Fatalf("в верхней части листа %d тёмных строк — похоже на вспомогательную линию", darkRows)
	}
}

// TestPreviewMarginsMatchRequested проверяет, что видимые отступы от края листа
// до цифр равны заданному значению — одинаково сверху и снизу. Раньше на A3
// (где масштаб миниатюры заметно меньше единицы) высота страницы передавалась
// в пикселях вместо миллиметров, и номер уезжал на 118 мм от нижнего края.
func TestPreviewMarginsMatchRequested(t *testing.T) {
	const boxW, boxH = 442, 325

	pages := []PagePreset{{Name: "A4", W: 210, H: 297}, {Name: "A3", W: 297, H: 420}, {Name: "Letter", W: 215.9, H: 279.4}}

	for _, page := range pages {
		for _, margin := range []float64{0, 5, 12, 25} {
			for _, size := range []float64{8, 12, 22, 60} {
				for _, row := range []int{RowTop, RowBottom} {
					opts := baseOptions()
					opts.PageWMM, opts.PageHMM = page.W, page.H
					opts.MarginXMM, opts.MarginYMM = 15, margin
					opts.FontSizePt, opts.Row = size, row
					opts.Prefix = "Стр."

					img := RenderPreview(opts, 1, boxW, boxH)
					pw, ph := opts.PageSizeMM()
					scale := math.Min(float64(boxW-24)/pw, float64(boxH-24)/ph)
					pageW := int(math.Round(pw * scale))
					pageH := int(math.Round(ph * scale))
					originX := int(math.Round(12 + (float64(boxW-24)-float64(pageW))/2))
					originY := int(math.Round(12 + (float64(boxH-24)-float64(pageH))/2))

					// порог выше обычного: ловим и сглаженные края цифры,
					// иначе измеренный зазор оказывается на 2-3 px больше
					// сканируем лист, не задевая его рамку, порог выше обычного:
					// так ловим и сглаженные края цифры
					scan := image.Rect(originX, originY, originX+pageW, originY+pageH).Inset(2)
					minY, maxY := math.MaxInt, math.MinInt
					for y := scan.Min.Y; y < scan.Max.Y; y++ {
						for x := scan.Min.X; x < scan.Max.X; x++ {
							r, g, b, _ := img.At(x, y).RGBA()
							if r>>8 < 215 && g>>8 < 215 && b>>8 < 215 {
								minY, maxY = min(minY, y), max(maxY, y)
							}
						}
					}
					if minY > maxY {
						t.Fatalf("%s, отступ %g, %gpt: номер не нарисован", page.Name, margin, size)
					}

					var gap float64
					if row == RowTop {
						gap = float64(minY-originY) / scale
					} else {
						gap = float64(originY+pageH-1-maxY) / scale
					}
					// допуск — округление до пикселя и запас в 1 пиксель от края
					tolerance := 1.0 + 2/scale
					if math.Abs(gap-margin) > tolerance {
						t.Errorf("%s, %s, отступ %.0f мм, кегль %gpt: зазор %.2f мм (допуск %.2f)",
							page.Name, []string{"сверху", "середина", "снизу"}[row], margin, size, gap, tolerance)
					}
				}
			}
		}
	}
}

// TestPreviewItalicIsApplied проверяет, что курсив в миниатюре действительно
// рисуется: раньше шрифт всегда брался обычным, и курсив было видно только в PDF.
func TestPreviewItalicIsApplied(t *testing.T) {
	opts := baseOptions()
	opts.Prefix = "Номер"
	opts.FontSizePt = 40

	_, regular := inkBounds(t, opts, 42, 445, 325)

	for _, tc := range []struct {
		name         string
		bold, italic bool
	}{
		{"курсив", false, true},
		{"жирный", true, false},
		{"жирный курсив", true, true},
	} {
		styled := opts
		styled.Bold, styled.Italic = tc.bold, tc.italic

		_, got := inkBounds(t, styled, 42, 445, 325)
		if got == regular {
			t.Errorf("%s: рисунок совпал с обычным начертанием — стиль не применён", tc.name)
		}
	}
}
