package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type cliFlags struct {
	out       string
	sequence  string
	preset    string
	widthMM   float64
	heightMM  float64
	landscape bool
	pos       string
	marginXMM float64
	marginYMM float64
	font      string
	sizePt    float64
	bold      bool
	italic    bool
	color     string
	prefix    string
	suffix    string
	dedupe    bool
}

// runCLI реализует консольный режим: удобно для проверки результата и
// использования в скриптах.
//
//	blank_num_gen -o out.pdf -s "1,2,6-9,42-43" -pos bottom-center
func runCLI(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("blank_num_gen", flag.ContinueOnError)
	fs.SetOutput(out)

	var f cliFlags
	fs.StringVar(&f.out, "o", "", "файл PDF (обязательный)")
	fs.StringVar(&f.sequence, "s", "1,2,6-9,42-43", "последовательность номеров, например 1,2,6-9,42-43")
	fs.StringVar(&f.preset, "size", "A4", "размер страницы: A4, Letter, Legal, A5 или WxH в мм, например 148x210")
	fs.BoolVar(&f.landscape, "landscape", false, "альбомная ориентация")
	fs.StringVar(&f.pos, "pos", "bottom-center", "положение номера: top|middle|bottom и left|center|right через дефис")
	fs.Float64Var(&f.marginXMM, "margin-x", 15, "отступ слева и справа, мм")
	fs.Float64Var(&f.marginYMM, "margin-y", 12, "отступ сверху и снизу, мм")
	fs.StringVar(&f.font, "font", "Helvetica", "шрифт: Helvetica, Times или Courier")
	fs.Float64Var(&f.sizePt, "pt", 12, "размер шрифта, pt")
	fs.BoolVar(&f.bold, "bold", false, "жирный шрифт")
	fs.BoolVar(&f.italic, "italic", false, "курсив")
	fs.StringVar(&f.color, "color", "black", "цвет номера: "+strings.Join(ColorNames(), ", "))
	fs.StringVar(&f.prefix, "prefix", "", "текст перед номером")
	fs.StringVar(&f.suffix, "suffix", "", "текст после номера")
	fs.BoolVar(&f.dedupe, "dedupe", false, "убирать повторяющиеся номера")
	fs.Usage = func() {
		fmt.Fprintln(out, "BlankNumGen — генератор пустых страниц PDF с номерами.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Использование:")
		fmt.Fprintln(out, "  blank_num_gen -o файл.pdf [флаги]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Флаги:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if f.out == "" {
		fs.Usage()
		return fmt.Errorf("не указан файл для сохранки (-o)")
	}

	opts, err := f.options()
	if err != nil {
		return err
	}

	file, err := os.Create(f.out)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := GeneratePDF(file, opts); err != nil {
		file.Close()
		os.Remove(f.out)
		return err
	}
	fmt.Fprintf(out, "Создано страниц: %d — %s\n", len(opts.Numbers), f.out)
	return nil
}

func (f *cliFlags) options() (GenerateOptions, error) {
	var o GenerateOptions

	numbers, err := ParseSequence(f.sequence, SeqOptions{RemoveDuplicates: f.dedupe})
	if err != nil {
		return o, err
	}

	w, h, err := parsePageSize(f.preset)
	if err != nil {
		return o, err
	}

	row, col, err := parsePosition(f.pos)
	if err != nil {
		return o, err
	}

	color, ok := colorsByName[f.color]
	if !ok {
		return o, fmt.Errorf("неизвестный цвет %q (доступно: %s)", f.color, strings.Join(ColorNames(), ", "))
	}

	o = GenerateOptions{
		Numbers:    numbers,
		PageWMM:    w,
		PageHMM:    h,
		Landscape:  f.landscape,
		MarginXMM:  f.marginXMM,
		MarginYMM:  f.marginYMM,
		Row:        row,
		Col:        col,
		FontFamily: f.font,
		FontSizePt: f.sizePt,
		Bold:       f.bold,
		Italic:     f.italic,
		Color:      color,
		Prefix:     f.prefix,
		Suffix:     f.suffix,
		Title:      "Страницы с номерами",
	}
	if err := o.Validate(); err != nil {
		return o, err
	}
	return o, nil
}

func parsePageSize(s string) (w, h float64, err error) {
	for _, p := range PagePresets {
		if strings.EqualFold(p.Name, s) {
			return p.W, p.H, nil
		}
	}
	s = strings.ReplaceAll(s, "мм", "")
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == 'x' || r == 'X' || r == '*'
	})
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("не удалось разобрать размер %q (например A4 или 148x210)", s)
	}
	w, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	h, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("не удалось разобрать размер %q (например A4 или 148x210)", s)
	}
	return w, h, nil
}

var rowNames = map[string]int{
	"top":      RowTop,
	"upper":    RowTop,
	"high":     RowTop,
	"верх":     RowTop,
	"сверху":   RowTop,
	"middle":   RowMiddle,
	"center":   RowMiddle,
	"centre":   RowMiddle,
	"середина": RowMiddle,
	"низ":      RowBottom,
	"bottom":   RowBottom,
	"lower":    RowBottom,
	"снизу":    RowBottom,
}

var colNames = map[string]int{
	"left":   ColLeft,
	"центр":  ColCenter,
	"слева":  ColLeft,
	"лево":   ColLeft,
	"center": ColCenter,
	"centre": ColCenter,
	"middle": ColCenter,
	"right":  ColRight,
	"справа": ColRight,
	"право":  ColRight,
}

// parsePosition разбирает положение номера: bottom-center, center-right и т.п.
// Первая часть отвечает за вертикаль, вторая — за горизонталь.
func parsePosition(s string) (row, col int, err error) {
	row, col = RowBottom, ColCenter

	parts := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == '-' || r == '_' || r == ',' || r == ' '
	})
	for i, part := range parts {
		if i == 0 {
			if r, ok := rowNames[part]; ok {
				row = r
				continue
			}
		}
		if i == 1 {
			if c, ok := colNames[part]; ok {
				col = c
				continue
			}
		}
		if r, ok := rowNames[part]; ok {
			row = r
			continue
		}
		if c, ok := colNames[part]; ok {
			col = c
			continue
		}
		return 0, 0, fmt.Errorf("не удалось разобрать положение %q (например bottom-center)", s)
	}
	return row, col, nil
}
