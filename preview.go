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

// minPreviewFontPx — минимальный кегль номера в миниатюре. На реальном листе
// A4 кегль 12 pt занимает около 5 пикселей, поэтому в превью цифра
// показывается крупнее; положение по базовой линии остаётся настоящим.
const minPreviewFontPx = 12.0

// drawNumber рисует номер страницы в выбранной позиции.
//
// Положение считается по-настоящему (в масштабе страницы), но сам номер
// рисуется минимальным читаемым кеглем, а тонкая линия показывает, где
// на самом деле окажется нижняя граница строки в PDF.
func drawNumber(img *image.RGBA, opts GenerateOptions, number int, originX, originY, pageW, pageH, scale float64, c color.RGBA) {
	label := LabelFor(number, opts.Prefix, opts.Suffix)
	if label == "" || opts.FontSizePt <= 0 {
		return
	}

	lineH := opts.LineHeightMM() * scale
	// центр настоящей строки текста на миниатюре
	centerY := originY + opts.RowY(opts.LineHeightMM(), pageH/scale)*scale + lineH/2

	trueFontPx := opts.FontSizePt * 25.4 / 72 * scale
	fontPx := math.Max(trueFontPx, minPreviewFontPx)

	face := newPreviewFace(opts.Bold, fontPx)
	if face == nil {
		return
	}
	defer face.Close()

	m := face.Metrics()
	// метрики face приходят в единицах 1/64 — переводим в пиксели
	ascent := float64(m.Ascent) / 64
	descent := float64(m.Descent) / 64

	textW := float64((&font.Drawer{Face: face}).MeasureString(label).Ceil())
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
	// номер не должен вылезать за пределы листа
	x = clampFloat(x, originX, originX+pageW-textW)

	// базовая линия увеличенного номера, прижатая к центру настоящей строки
	baseline := centerY + (descent-ascent)/2

	// и номер, и направляющая остаются внутри листа, даже если отступ равен
	// нулю и настоящая строка упирается в самый край страницы
	const inset = 1.0
	minBaseline := originY + ascent + inset
	maxBaseline := originY + pageH - descent - inset
	if maxBaseline < minBaseline {
		baseline = clampFloat(centerY, originY+1, originY+pageH-2)
	} else {
		baseline = clampFloat(baseline, minBaseline, maxBaseline)
	}

	if !sameFontSize(trueFontPx, fontPx) {
		// направляющая: где в PDF будет низ настоящей строки
		trueBaseline := clampFloat(trueBaselineY(opts, scale, pageH)+originY,
			originY+1, originY+pageH-2)
		line := color.NRGBA{c.R, c.G, c.B, 0x60}
		guide := image.Rect(
			int(math.Round(x)),
			int(math.Round(trueBaseline)),
			int(math.Round(x+textW))+1,
			int(math.Round(trueBaseline))+1,
		)
		// при нулевых отступах линия упирается в край — обрезаем по листу
		guide = guide.Intersect(image.Rect(
			int(originX), int(originY), int(originX+pageW), int(originY+pageH)))
		draw.Draw(img, guide, image.NewUniform(line), image.Point{}, draw.Src)
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(int(math.Round(x))),
			Y: fixed.I(int(math.Round(baseline))),
		},
	}
	d.DrawString(label)
}

// trueBaselineY возвращает положение нижней границы настоящей строки текста
// относительно верха листа, в пикселях миниатюры.
func trueBaselineY(opts GenerateOptions, scale, pageHeightMM float64) float64 {
	lh := opts.LineHeightMM()
	bottom := opts.RowY(lh, pageHeightMM) + lh // низ строки
	return bottom * scale
}

func sameFontSize(truePx, px float64) bool {
	return math.Abs(truePx-px) < 0.01
}

func drawRectBorder(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
}
