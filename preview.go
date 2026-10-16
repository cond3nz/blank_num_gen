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
	previewParsed map[fontStyleKey]*opentype.Font
)

// fontStyleKey — начертание для миниатюры.
type fontStyleKey struct {
	bold   bool
	italic bool
}

func previewFonts() map[fontStyleKey]*opentype.Font {
	previewOnce.Do(func() {
		previewParsed = make(map[fontStyleKey]*opentype.Font, 4)
		for _, key := range []fontStyleKey{
			{},
			{bold: true},
			{italic: true},
			{bold: true, italic: true},
		} {
			data := dejavuSans
			switch {
			case key.bold && key.italic:
				data = dejavuSansBoldOblique
			case key.bold:
				data = dejavuSansBold
			case key.italic:
				data = dejavuSansOblique
			}
			if f, err := opentype.Parse(data); err == nil {
				previewParsed[key] = f
			}
		}
	})
	return previewParsed
}

func newPreviewFace(bold, italic bool, sizePx float64) font.Face {
	f, ok := previewFonts()[fontStyleKey{bold: bold, italic: italic}]
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
	drawNumber(img, opts, number, originX, originY, pageW, pageH, ph, scale,
		color.RGBA{uint8(c.R), uint8(c.G), uint8(c.B), 0xFF})

	border := color.RGBA{0x9A, 0x9A, 0x9A, 0xFF}
	drawRectBorder(img, pageRect, border)

	return img
}

// minPreviewFontPx — минимальный кегль номера в миниатюре. На реальном листе
// A4 кегль 12 pt занимает около 5 пикселей и не читается, поэтому в превью
// цифра показывается крупнее, оставаясь на настоящей базовой линии.
const minPreviewFontPx = 12.0

// Пропорции цифр DejaVu Sans: высота цифры и спуск в долях кегля. Такими же
// пропорциями пользуется pdfgen для настоящего размера цифры в PDF, поэтому
// отступы от края до края рисунка совпадают в миниатюре и в готовом файле.
const (
	digitCapRatio     = 0.729 // высота цифр
	digitDescentRatio = 0.236 // спуск под базовой линией
)

// previewBaseline — базовая линия увеличенной цифры в координатах миниатюры.
// Отступы отсчитываются до краёв рисунка: сверху — от верха цифр, снизу — от
// низа цифр, поэтому при одинаковом значении отступы выглядят одинаково и не
// зависят от кегля.
func previewBaseline(opts GenerateOptions, originY, scale, fontPx float64) float64 {
	_, ph := opts.PageSizeMM()
	cap := digitCapRatio * fontPx
	descent := digitDescentRatio * fontPx

	switch opts.Row {
	case RowMiddle:
		return originY + (ph/2)*scale + cap/2
	case RowBottom:
		return originY + (ph-opts.MarginYMM)*scale - descent
	default:
		return originY + opts.MarginYMM*scale + cap
	}
}

// drawNumber рисует номер страницы в выбранной позиции.
//
// Положение считается по-настоящему (в масштабе страницы): базовая линия
// цифры совпадает с настоящей базовой линией строки в PDF. Чтобы номер был
// читаем в маленькой миниатюре, цифра рисуется минимальным читаемым кеглем —
// она растёт вверх от настоящей линии, а не смещается относительно неё.
func drawNumber(img *image.RGBA, opts GenerateOptions, number int, originX, originY, pageW, pageH, pageHeightMM, scale float64, c color.RGBA) {
	label := LabelFor(number, opts.Prefix, opts.Suffix)
	if label == "" || opts.FontSizePt <= 0 {
		return
	}

	// увеличенный кегль нужен только для читаемости
	fontPx := math.Max(opts.FontSizePt*25.4/72*scale, minPreviewFontPx)

	face := newPreviewFace(opts.Bold, opts.Italic, fontPx)
	if face == nil {
		return
	}
	defer face.Close()

	m := face.Metrics()
	// метрики face приходят в единицах 1/64 — переводим в пиксели
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

	// Цифра увеличена до читаемого размера, но её базовая линия совпадает с
	// настоящей: чем крупнее кегль, тем выше растёт цифра от этой линии, а
	// не «отъезжает» от неё. Если прижатая к краю строка вылезла бы за лист,
	// номер сдвигается внутрь — иначе он не помещался бы в страницу вовсе.
	// Цифры не поднимаются выше высоты прописной (0.729 кегля); запас нужен
	// на выносные элементы строчных букв в префиксе/суффиксе.
	const inset = 1.0
	baseline := previewBaseline(opts, originY, scale, fontPx)
	minBaseline := originY + 0.8*fontPx + inset
	maxBaseline := originY + pageH - descent*1.05 - inset
	if maxBaseline < minBaseline {
		baseline = clampFloat(baseline, originY+1, originY+pageH-2)
	} else {
		baseline = clampFloat(baseline, minBaseline, maxBaseline)
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

func drawRectBorder(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
}
