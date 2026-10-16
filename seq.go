package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxPages ограничивает количество страниц в одном PDF-файле.
const MaxPages = 50000

// ErrEmptySequence возвращается, когда в поле последовательности ничего нет.
var ErrEmptySequence = errors.New("последовательность пуста — введите, например, 1,2,6-9,42-43")

// SeqOptions — параметры разбора последовательности.
type SeqOptions struct {
	// RemoveDuplicates убирает повторяющиеся номера, сохраняя первое вхождение.
	RemoveDuplicates bool
	// MaxPages ограничивает количество страниц. Значение <= 0 означает MaxPages.
	MaxPages int
}

// разделители диапазонов: дефис, минус, короткое и длинное тире.
var rangeDashes = []rune{'-', '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2212'}

// ParseSequence разбирает строку вида "1,2,6-9,42-43" в список номеров страниц.
//
// Поддерживаются:
//   - отдельные номера: 1, 2, 42
//   - диапазоны: 6-9 (а также 9-6 — в обратном порядке)
//   - разделители: запятая, точка с запятой, перевод строки
//   - произвольные пробелы внутри элементов
func ParseSequence(s string, opts SeqOptions) ([]int, error) {
	max := opts.MaxPages
	if max <= 0 {
		max = MaxPages
	}

	fields := splitFields(s)
	if len(fields) == 0 {
		return nil, ErrEmptySequence
	}

	numbers := make([]int, 0, len(fields))
	seen := make(map[int]bool)

	add := func(n int) error {
		if n < 0 {
			return fmt.Errorf("номер страницы не может быть отрицательным: %d", n)
		}
		if opts.RemoveDuplicates {
			if seen[n] {
				return nil
			}
			seen[n] = true
		}
		numbers = append(numbers, n)
		if len(numbers) > max {
			return fmt.Errorf("слишком много страниц: больше %d", max)
		}
		return nil
	}

	for _, field := range fields {
		lo, hi, isRange, err := parseField(field)
		if err != nil {
			return nil, err
		}
		if !isRange {
			if err := add(lo); err != nil {
				return nil, err
			}
			continue
		}

		step := 1
		if lo > hi {
			step = -1
		}
		count := (hi - lo) * step
		if count+1 > max {
			return nil, fmt.Errorf("диапазон %q слишком большой: %d страниц", field, count+1)
		}
		for n := lo; ; n += step {
			if err := add(n); err != nil {
				return nil, err
			}
			if n == hi {
				break
			}
		}
	}

	if len(numbers) == 0 {
		return nil, ErrEmptySequence
	}
	return numbers, nil
}

// splitFields разбивает строку на элементы по запятым, точкам с запятой и
// переводам строк. Пустые элементы игнорируются.
func splitFields(s string) []string {
	fields := make([]string, 0, 8)
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		if f := strings.TrimSpace(part); f != "" {
			fields = append(fields, f)
		}
	}
	return fields
}

// parseField разбирает один элемент последовательности.
func parseField(field string) (lo, hi int, isRange bool, err error) {
	if idx, width := indexOfDash(field); idx >= 0 {
		left := strings.TrimSpace(field[:idx])
		right := strings.TrimSpace(field[idx+width:])
		if left == "" || right == "" {
			return 0, 0, true, fmt.Errorf("неполный диапазон %q: ожидается, например, 6-9", field)
		}
		lo, err = strconv.Atoi(left)
		if err != nil {
			return 0, 0, true, fmt.Errorf("не удалось разобрать %q в диапазоне %q", left, field)
		}
		hi, err = strconv.Atoi(right)
		if err != nil {
			return 0, 0, true, fmt.Errorf("не удалось разобрать %q в диапазоне %q", right, field)
		}
		return lo, hi, true, nil
	}

	n, err := strconv.Atoi(strings.TrimSpace(field))
	if err != nil {
		return 0, 0, false, fmt.Errorf("не удалось разобрать %q: ожидается номер или диапазон, например 6-9", field)
	}
	return n, n, false, nil
}

// indexOfDash возвращает байтовое смещение тире и его длину в байтах.
func indexOfDash(s string) (idx, width int) {
	for i, r := range s {
		for _, d := range rangeDashes {
			if r == d {
				return i, utf8.RuneLen(d)
			}
		}
	}
	return -1, 0
}
