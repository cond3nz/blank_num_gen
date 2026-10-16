package main

import (
	"fmt"
	"math"
	"strings"
)

// Приблизительная ширина текста в мм для шрифта DejaVu.
// Используется только для миниатюры в GUI: точные метрики считать незачем.
func approxTextWidthMM(text string, sizePt float64) float64 {
	// средняя ширина символа DejaVu Sans ≈ 0.55 em, кириллица чуть шире
	var em float64
	for _, r := range text {
		switch {
		case r == ' ':
			em += 0.32
		case r >= 0x0400: // кириллица
			em += 0.60
		case strings.ContainsRune(".,:;!|'`", r):
			em += 0.30
		default:
			em += 0.56
		}
	}
	return em * sizePt * 25.4 / 72
}

func clampFloat(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// shortRange описывает список номеров компактно: "1–9, 42".
func shortRange(numbers []int) string {
	if len(numbers) == 0 {
		return ""
	}
	parts := make([]string, 0, 4)
	start, prev := numbers[0], numbers[0]
	flush := func() {
		switch {
		case prev == start:
			parts = append(parts, fmt.Sprint(start))
		case prev == start+1:
			parts = append(parts, fmt.Sprintf("%d, %d", start, prev))
		default:
			parts = append(parts, fmt.Sprintf("%d–%d", start, prev))
		}
	}
	for _, n := range numbers[1:] {
		if n == prev+1 {
			prev = n
			continue
		}
		flush()
		start, prev = n, n
	}
	flush()
	return strings.Join(parts, ", ")
}
