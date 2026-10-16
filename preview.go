package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Встроенные шрифты используются для миниатюры: стандартные шрифты PDF
// не содержат кириллицы, а на экране она может быть в префиксе/суффиксе.

var (
	previewOnce   sync.Once
	previewParsed map[bool]*opentype.Font
)

func previewFonts() map[bool]*opentype.Font {
	previewOnce.Do(func() {
		previewParsed = make(map[bool]*opentype.Font, 2)
		for _, bold := range []bool{false, true} {
			data := dejavuSans
			if bold {
				data = dejavuSansBold
			}
			if f, err := opentype.Parse(data); err == nil {
				previewParsed[bold] = f
			}
		}
	})
	return previewParsed
}

func newPreviewFace(bold bool, sizePx float64) font.Face {
	f, ok := previewFonts()[bold]
	if !ok {
		return nil
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: sizePx, DPI: 72})
	if err != nil {
		return nil
	}
	return face
}

// RenderPreview рисует миниатюру страницы: белый лист с номером в выбранной
// позиции. Возвращает nil, если отрисовать нечего.
func RenderPreview(opts GenerateOptions, number int, boxW, boxH int) image.Image {
	if boxW < 20 || boxH < 20 {
		return nil
	}

	pw, ph := opts.PageSizeMM()
	if pw <= 0 || ph <= 0 {
		return nil
	}

	const pad = 12
	availW := float64(boxW - 2*pad)
	availH := float64(boxH - 2*pad)
	scale := math.Min(availW/pw, availH/ph)
	if scale <= 0 {
		return nil
	}

	pageW := pw * scale
	pageH := ph * scale
	originX := float64(pad) + (availW-pageW)/2
	originY := float64(pad) + (availH-pageH)/2

	img := image.NewRGBA(image.Rect(0, 0, boxW, boxH))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0xE6, 0xE8, 0xEC, 0xFF}), image.Point{}, draw.Src)

	pageRect := image.Rect(
		int(math.Round(originX)),
		int(math.Round(originY)),
		int(math.Round(originX+pageW)),
		int(math.Round(originY+pageH)),
	)

	shadow := image.Rect(pageRect.Min.X+3, pageRect.Min.Y+3, pageRect.Max.X+3, pageRect.Max.Y+3)
	draw.Draw(img, shadow, image.NewUniform(color.RGBA{0xC8, 0xCB, 0xD2, 0xFF}), image.Point{}, draw.Src)
	draw.Draw(img, pageRect, image.NewUniform(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}), image.Point{}, draw.Src)

	c := opts.Color
	drawNumber(img, opts, number, originX, originY, pageW, pageH, scale,
		color.RGBA{uint8(c.R), uint8(c.G), uint8(c.B), 0xFF})

	border := color.RGBA{0x9A, 0x9A, 0x9A, 0xFF}
	drawRectBorder(img, pageRect, border)

	return img
}

func drawNumber(img *image.RGBA, opts GenerateOptions, number int, originX, originY, pageW, pageH, scale float64, c color.RGBA) {
	label := LabelFor(number, opts.Prefix, opts.Suffix)
	if label == "" || opts.FontSizePt <= 0 {
		return
	}

	lineH := opts.LineHeightMM() * scale
	centerY := originY + opts.RowY(opts.LineHeightMM(), pageH/scale)*scale + lineH/2

	fontPx := opts.FontSizePt * 25.4 / 72 * scale

	// Ширина текста: если шрифт достаточно крупный, считаем по метрикам,
	// иначе используем приближение — важен только факт положения.
	textW := approxTextWidthMM(label, opts.FontSizePt) * scale
	if fontPx >= 7 {
		if face := newPreviewFace(opts.Bold, fontPx); face != nil {
			defer face.Close()
			d := &font.Drawer{Face: face}
			textW = float64(d.MeasureString(label).Ceil())
		}
	}
	if textW <= 0 {
		return
	}

	left := originX + opts.MarginXMM*scale
	right := originX + pageW - opts.MarginXMM*scale
	if right-left < textW {
		left = originX + (pageW-textW)/2
	}

	var x float64
	switch opts.Col {
	case ColCenter:
		x = (left+right)/2 - textW/2
	case ColRight:
		x = right - textW
	default:
		x = left
	}
	if x < originX {
		x = originX
	}

	if fontPx < 7 {
		// слишком мелко для чтения — рисуем полоску той же ширины
		barH := math.Max(2, math.Min(lineH*0.6, fontPx*0.9))
		rect := image.Rect(
			int(math.Round(x)),
			int(math.Round(centerY-barH/2)),
			int(math.Round(x+textW))+1,
			int(math.Round(centerY+barH/2))+1,
		)
		draw.Draw(img, rect, image.NewUniform(c), image.Point{}, draw.Src)
		return
	}

	face := newPreviewFace(opts.Bold, fontPx)
	if face == nil {
		return
	}
	defer face.Close()

	m := face.Metrics()
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(int(math.Round(x))),
			Y: fixed.I(int(math.Round(centerY + (float64(m.Descent)-float64(m.Ascent))/2))),
		},
	}
	d.DrawString(label)
}

func drawRectBorder(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
}
