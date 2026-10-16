package main

import (
	"reflect"
	"testing"
)

func TestParseSequence(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts SeqOptions
		want []int
	}{
		{"пример из задания", "1,2,6-9,42-43", SeqOptions{}, []int{1, 2, 6, 7, 8, 9, 42, 43}},
		{"одиночное число", "7", SeqOptions{}, []int{7}},
		{"пробелы", " 1 , 2 , 6 - 9 ", SeqOptions{}, []int{1, 2, 6, 7, 8, 9}},
		{"перевод строк", "1\n2\n3", SeqOptions{}, []int{1, 2, 3}},
		{"точки с запятой", "1;2;3", SeqOptions{}, []int{1, 2, 3}},
		{"обратный диапазон", "9-6", SeqOptions{}, []int{9, 8, 7, 6}},
		{"диапазон из одного", "5-5", SeqOptions{}, []int{5}},
		{"длинное тире", "1–4", SeqOptions{}, []int{1, 2, 3, 4}},
		{"ноль", "0,1", SeqOptions{}, []int{0, 1}},
		{"повторы оставлены", "1,2,2,3", SeqOptions{}, []int{1, 2, 2, 3}},
		{"повторы убраны", "1,2,2,3", SeqOptions{RemoveDuplicates: true}, []int{1, 2, 3}},
		{"повтор диапазона", "1-3,3-5", SeqOptions{RemoveDuplicates: true}, []int{1, 2, 3, 4, 5}},
		{"много элементов", "1,2,3,4,5,6,7,8,9,10", SeqOptions{}, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSequence(tt.in, tt.opts)
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("получено %v, ожидалось %v", got, tt.want)
			}
		})
	}
}

func TestParseSequenceErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"пусто", ""},
		{"пробелы", "   "},
		{"запятые", ",,,"},
		{"не число", "abc"},
		{"буква в диапазоне", "a-b"},
		{"неполный диапазон", "6-"},
		{"диапазон без начала", "-6"},
		{"отрицательное число", "-5"},
		{"слишком большой диапазон", "1-100000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseSequence(tt.in, SeqOptions{}); err == nil {
				t.Fatalf("ожидалась ошибка для %q", tt.in)
			}
		})
	}
}

func TestParseSequenceMaxPages(t *testing.T) {
	if _, err := ParseSequence("1-10", SeqOptions{MaxPages: 5}); err == nil {
		t.Fatal("ожидалась ошибка: страниц больше, чем MaxPages")
	}
	if got, err := ParseSequence("1-5", SeqOptions{MaxPages: 5}); err != nil || len(got) != 5 {
		t.Fatalf("получено %v, %v", got, err)
	}
}

func TestShortRange(t *testing.T) {
	tests := []struct {
		in   []int
		want string
	}{
		{[]int{1, 2, 6, 7, 8, 9, 42, 43}, "1, 2, 6–9, 42, 43"},
		{[]int{1, 2, 3, 4}, "1–4"},
		{[]int{5}, "5"},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := shortRange(tt.in); got != tt.want {
			t.Errorf("shortRange(%v) = %q, ожидалось %q", tt.in, got, tt.want)
		}
	}
}
