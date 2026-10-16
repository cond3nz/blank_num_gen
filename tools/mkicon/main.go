// Command mkicon генерирует app.ico — иконку приложения: пустой лист
// с загнутым углом и цифрой внизу по центру (как это делает само приложение).
//
//	go run ./tools/mkicon -out app.ico
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var sizes = []int{16, 24, 32, 48, 64, 128, 256}

var (
	pageWhite  = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	pageBorder = color.NRGBA{0xB0, 0xB7, 0xC4, 0xFF}
	foldFill   = color.NRGBA{0xE4, 0xE8, 0xEF, 0xFF}
	foldEdge   = color.NRGBA{0xC2, 0xC9, 0xD6, 0xFF}
	shadow     = color.NRGBA{0x1B, 0x22, 0x33, 0x38}
	digitColor = color.NRGBA{0x1F, 0x2A, 0x44, 0xFF}
)

func main() {
	out := flag.String("out", "app.ico", "файл .ico")
	fontPath := flag.String("font", "assets/fonts/DejaVuSans-Bold.ttf", "шрифт с цифрой")
	flag.Parse()

	fontData, err := os.ReadFile(*fontPath)
	if err != nil {
		fatal(err)
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		fatal(err)
	}

	images := make([]image.Image, 0, len(sizes))
	for _, size := range sizes {
		images = append(images, renderIcon(parsed, size))
	}

	data, err := encodeICO(images)
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("%s: %d КБ, размеры %v\n", *out, len(data)/1024, sizes)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "mkicon:", err)
	os.Exit(1)
}

// renderIcon рисует иконку заданного размера в пикселях.
func renderIcon(fnt *opentype.Font, size int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size)

	const (
		marginX = 0.12
		marginY = 0.08
		shift   = 0.05 // смещение тени
		radius  = 0.10 // скругление углов
	)

	pageX0 := marginX * s
	pageY0 := marginY * s
	pageX1 := s - marginX*s
	pageY1 := s - marginY*s - shift*s
	rad := radius * s
	line := math.Max(1, s*0.018)

	// тень
	fillRoundedRect(img, pageX0+shift*s, pageY0+shift*s, pageX1+shift*s, pageY1+shift*s, rad, shadow)
	// лист: рамка + белая заливка
	fillRoundedRect(img, pageX0, pageY0, pageX1, pageY1, rad, pageBorder)
	fillRoundedRect(img, pageX0+line, pageY0+line, pageX1-line, pageY1-line, rad*0.75, pageWhite)

	fold := (pageX1 - pageX0) * 0.3
	cutCorner(img, pageX0, pageY0, pageX1, fold) // уголок листа срезан
	fillTriangle(img, pageX1-fold, pageY0, pageX1-fold, pageY0+fold, pageX1, pageY0+fold, foldFill)
	drawLine(img, pageX1-fold, pageY0, pageX1, pageY0+fold, foldEdge, line)
	drawLine(img, pageX1-fold, pageY0+fold, pageX1, pageY0+fold, foldEdge, line)

	drawDigit(img, fnt, s, pageX0, pageX1, pageY1)

	return img
}

// drawDigit рисует цифру «1» по центру нижней части листа.
func drawDigit(img *image.NRGBA, fnt *opentype.Font, s, pageX0, pageX1, pageY1 float64) {
	sizePx := s * 0.42
	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{Size: sizePx, DPI: 72})
	if err != nil {
		return
	}
	defer face.Close()

	const text = "1"
	width := float64((&font.Drawer{Face: face}).MeasureString(text).Ceil())
	m := face.Metrics()

	centerX := (pageX0 + pageX1) / 2
	bottom := pageY1 - s*0.09
	baseline := bottom + float64(m.Descent)/64

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(digitColor),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(int(math.Round(centerX - width/2))),
			Y: fixed.I(int(math.Round(baseline))),
		},
	}
	d.DrawString(text)
}

// fillRoundedRect заливает прямоугольник со скруглёнными углами.
func fillRoundedRect(img *image.NRGBA, x0, y0, x1, y1, r float64, c color.NRGBA) {
	b := img.Bounds()
	if r > (x1-x0)/2 {
		r = (x1 - x0) / 2
	}
	if r > (y1-y0)/2 {
		r = (y1 - y0) / 2
	}

	for y := int(math.Floor(y0)); y <= int(math.Ceil(y1)); y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		py := float64(y) + 0.5
		dy := math.Max(math.Max(y0+r-py, 0), py-(y1-r))

		for x := int(math.Floor(x0)); x <= int(math.Ceil(x1)); x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			px := float64(x) + 0.5
			dx := math.Max(math.Max(x0+r-px, 0), px-(x1-r))

			if dx*dx+dy*dy <= r*r+0.01 {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

// cutCorner убирает (делает прозрачным) треугольник загнутого угла.
func cutCorner(img *image.NRGBA, x0, y0, x1, fold float64) {
	b := img.Bounds()
	for y := int(y0); y < int(y0+fold); y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		cut := int(math.Round(x1 - (float64(y) - y0)))
		for x := cut; x < int(x1); x++ {
			if x >= b.Min.X && x < b.Max.X {
				img.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
}

// fillTriangle заливает треугольник p0-p1-p2.
func fillTriangle(img *image.NRGBA, p0x, p0y, p1x, p1y, p2x, p2y float64, c color.NRGBA) {
	b := img.Bounds()
	minY := math.Min(p0y, math.Min(p1y, p2y))
	maxY := math.Max(p0y, math.Max(p1y, p2y))
	minX := math.Min(p0x, math.Min(p1x, p2x))
	maxX := math.Max(p0x, math.Max(p1x, p2x))

	area := (p1x-p0x)*(p2y-p0y) - (p2x-p0x)*(p1y-p0y)
	if area == 0 {
		return
	}

	for y := int(minY); y <= int(maxY); y++ {
		py := float64(y) + 0.5
		for x := int(minX); x <= int(maxX); x++ {
			px := float64(x) + 0.5

			w0 := ((p1x-px)*(p2y-py) - (p2x-px)*(p1y-py)) / area
			w1 := ((p2x-px)*(p0y-py) - (p0x-px)*(p2y-py)) / area
			w2 := ((p0x-px)*(p1y-py) - (p1x-px)*(p0y-py)) / area

			if w0 >= -0.01 && w1 >= -0.01 && w2 >= -0.01 {
				if x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y {
					img.SetNRGBA(x, y, c)
				}
			}
		}
	}
}

// drawLine рисует линию заданной толщины (толщина задаётся сдвигом по X).
func drawLine(img *image.NRGBA, x0, y0, x1, y1 float64, c color.NRGBA, thickness float64) {
	steps := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := x0 + (x1-x0)*t
		py := y0 + (y1-y0)*t
		for dx := 0; dx < int(math.Ceil(thickness)); dx++ {
			for dy := 0; dy < int(math.Ceil(thickness)); dy++ {
				x, y := int(px)+dx, int(py)+dy
				if image.Pt(x, y).In(img.Bounds()) {
					img.SetNRGBA(x, y, c)
				}
			}
		}
	}
}

// encodeICO собирает ICO из готовых PNG-картинок.
func encodeICO(images []image.Image) ([]byte, error) {
	var buf bytes.Buffer

	pngs := make([][]byte, len(images))
	// картинки начинаются сразу после каталога
	total := 6 + 16*len(images)
	for i, img := range images {
		var pb bytes.Buffer
		if err := png.Encode(&pb, img); err != nil {
			return nil, err
		}
		pngs[i] = pb.Bytes()
	}

	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // тип: иконка
	binary.Write(&buf, binary.LittleEndian, uint16(len(images)))

	offset := total
	for i, img := range images {
		b := img.Bounds()
		width, height := b.Dx(), b.Dy()
		if width > 255 {
			width = 0 // 0 означает 256
		}
		if height > 255 {
			height = 0
		}
		buf.WriteByte(byte(width))
		buf.WriteByte(byte(height))
		buf.WriteByte(0)                                    // палитра не используется
		buf.WriteByte(0)                                    // зарезервировано
		binary.Write(&buf, binary.LittleEndian, uint16(1))  // цветовые плоскости
		binary.Write(&buf, binary.LittleEndian, uint16(32)) // бит на пиксель
		binary.Write(&buf, binary.LittleEndian, uint32(len(pngs[i])))
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(pngs[i])
	}

	for _, p := range pngs {
		buf.Write(p)
	}

	return buf.Bytes(), nil
}
