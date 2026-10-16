package main

import (
	"image"
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
