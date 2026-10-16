package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"
)

func TestRenderIconSizes(t *testing.T) {
	parsed := testFont(t)

	for _, size := range sizes {
		img := renderIcon(parsed, size)

		if got := img.Bounds(); got.Dx() != size || got.Dy() != size {
			t.Fatalf("размер %d: получено %v", size, got)
		}

		// на иконке должны быть непрозрачные пиксели (лист и цифра)
		opaque := 0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0x8000 {
					opaque++
				}
			}
		}
		if opaque < size*size/4 {
			t.Fatalf("размер %d: непрозрачных пикселей всего %d", size, opaque)
		}

		// прозрачный фон по углам
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatalf("размер %d: угол должен быть прозрачным", size)
		}
	}
}

func TestEncodeICO(t *testing.T) {
	parsed := testFont(t)

	images := make([]image.Image, 0, len(sizes))
	for _, size := range sizes {
		images = append(images, renderIcon(parsed, size))
	}

	data, err := encodeICO(images)
	if err != nil {
		t.Fatalf("encodeICO: %v", err)
	}

	if len(data) < 6+16*len(images) {
		t.Fatalf("файл слишком короткий: %d байт", len(data))
	}
	if reserved := binary.LittleEndian.Uint16(data[0:2]); reserved != 0 {
		t.Fatalf("reserved = %d, ожидалось 0", reserved)
	}
	if kind := binary.LittleEndian.Uint16(data[2:4]); kind != 1 {
		t.Fatalf("тип = %d, ожидалась иконка (1)", kind)
	}
	if count := binary.LittleEndian.Uint16(data[4:6]); int(count) != len(images) {
		t.Fatalf("количество картинок = %d, ожидалось %d", count, len(images))
	}

	// записи каталога должны идти подряд и начинаться сразу после каталога
	offset := 6 + 16*len(images)
	for i := range images {
		entry := data[6+16*i : 6+16*(i+1)]

		wantWidth := byte(sizes[i])
		if sizes[i] == 256 {
			wantWidth = 0 // в ICO размер 256 кодируется нулём
		}
		if entry[0] != wantWidth {
			t.Fatalf("картинка %d: ширина = %d, ожидалось %d", i, entry[0], wantWidth)
		}
		if entry[1] != wantWidth {
			t.Fatalf("картинка %d: высота = %d, ожидалось %d", i, entry[1], wantWidth)
		}
		if planes := binary.LittleEndian.Uint16(entry[4:6]); planes != 1 {
			t.Fatalf("картинка %d: цветовых плоскостей %d, ожидалась 1", i, planes)
		}

		size := int(binary.LittleEndian.Uint32(entry[8:12]))
		start := int(binary.LittleEndian.Uint32(entry[12:16]))
		if start != offset {
			t.Fatalf("картинка %d: смещение %d, ожидалось %d", i, start, offset)
		}
		if start+size > len(data) {
			t.Fatalf("картинка %d: данные выходят за пределы файла", i)
		}

		// содержимое должно распаковываться как PNG
		if _, err := png.Decode(bytes.NewReader(data[start : start+size])); err != nil {
			t.Fatalf("картинка %d: %v", i, err)
		}
		offset += size
	}

	if offset != len(data) {
		t.Fatalf("после картинок осталось %d лишних байт", len(data)-offset)
	}
}

func TestRenderIconWithoutFont(t *testing.T) {
	// при нечитаемом шрифте картинка всё равно должна собираться (лист без цифры)
	img := renderIcon(nil, 64)
	if img == nil {
		t.Fatal("миниатюра не построена")
	}
}
