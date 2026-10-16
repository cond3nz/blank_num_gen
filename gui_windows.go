//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
)

const appTitle = "BlankNumGen — пустые страницы с номерами"

const prefixHint = "Текст перед номером на странице. Например «Стр. » даст «Стр. 42», " +
	"а «— » даст «— 42». Пустое поле — на странице будет только номер."

const suffixHint = "Текст после номера на странице. Например « —» даст «42 —», " +
	"а «/2019» даст «42/2019». Пустое поле — на странице будет только номер."

// appUI — окно приложения.
type appUI struct {
	mw *walk.MainWindow

	sequence *walk.LineEdit
	dedupe   *walk.CheckBox
	status   *walk.Label

	preset    *walk.ComboBox
	pageW     *walk.NumberEdit
	pageH     *walk.NumberEdit
	landscape *walk.CheckBox

	colButtons []*walk.RadioButton
	rowButtons []*walk.RadioButton
	marginX    *walk.NumberEdit
	marginY    *walk.NumberEdit

	fontFamily *walk.ComboBox
	fontSize   *walk.NumberEdit
	bold       *walk.CheckBox
	italic     *walk.CheckBox
	color      *walk.ComboBox
	prefix     *walk.LineEdit
	suffix     *walk.LineEdit
	openAfter  *walk.CheckBox

	preview *walk.ImageView

	// состояние последней нарисованной картинки: не перерисовываем зря
	previewW, previewH int
	previewOpts        string

	lastDir string
}

// runGUI запускает графическое приложение.
func runGUI() error {
	ui := &appUI{}

	mw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	ui.mw = mw

	cfg := LoadSettings()
	ui.lastDir = cfg.LastDir

	if err := ui.build(cfg); err != nil {
		mw.Dispose()
		return err
	}

	ui.refresh()

	// В императивном API walk окно создаётся скрытым: без Show() процесс
	// просто живёт в цикле сообщений, а окно не появляется.
	mw.Show()

	mw.Run()

	if err := SaveSettings(ui.settings()); err != nil {
		walk.MsgBox(nil, appTitle, "Не удалось сохранить настройки:\n"+err.Error(), walk.MsgBoxIconWarning)
	}
	return nil
}

// showFatalError показывает ошибку приложения.
func showFatalError(err error) {
	walk.MsgBox(nil, appTitle, err.Error(), walk.MsgBoxIconError)
}

// ---------- построение интерфейса ----------

func (ui *appUI) build(cfg AppSettings) error {
	ui.mw.SetTitle(appTitle)
	if err := ui.mw.SetSize(walk.Size{Width: 960, Height: 720}); err != nil {
		return err
	}
	if err := ui.mw.SetMinMaxSize(
		walk.Size{Width: 960, Height: 660},
		walk.Size{Width: 2400, Height: 1600},
	); err != nil {
		return err
	}

	root, err := vboxLayout(9, 8)
	if err != nil {
		return err
	}
	if err := ui.mw.SetLayout(root); err != nil {
		return err
	}

	content, contentLayout, err := ui.newHBox(ui.mw, 10)
	if err != nil {
		return err
	}

	left, err := ui.newVBox(content, 10)
	if err != nil {
		return err
	}

	right, err := ui.newVBox(content, 10)
	if err != nil {
		return err
	}

	if err := ui.buildSequenceGroup(left, cfg); err != nil {
		return err
	}
	if err := ui.buildPageGroup(left, cfg); err != nil {
		return err
	}
	if err := ui.buildFontGroup(left, cfg); err != nil {
		return err
	}
	if err := ui.buildPositionGroup(right, cfg); err != nil {
		return err
	}
	if err := ui.buildPreview(right); err != nil {
		return err
	}

	if err := contentLayout.SetStretchFactor(left, 3); err != nil {
		return err
	}
	if err := contentLayout.SetStretchFactor(right, 2); err != nil {
		return err
	}

	hint, err := walk.NewLabel(ui.mw)
	if err != nil {
		return err
	}
	if err := hint.SetText("Последовательность: номера через запятую и диапазоны через дефис — например 1,2,6-9,42-43. Каждый номер становится отдельной пустой страницей."); err != nil {
		return err
	}

	ui.status, err = walk.NewLabel(ui.mw)
	if err != nil {
		return err
	}
	if err := ui.status.SetText(""); err != nil {
		return err
	}

	footer, _, err := ui.newHBox(ui.mw, 8)
	if err != nil {
		return err
	}

	createBtn, err := walk.NewPushButton(footer)
	if err != nil {
		return err
	}
	if err := createBtn.SetText("Создать PDF…"); err != nil {
		return err
	}
	if err := createBtn.SetMinMaxSize(walk.Size{Width: 150}, walk.Size{}); err != nil {
		return err
	}
	createBtn.Clicked().Attach(ui.onCreateClicked)

	ui.openAfter, err = walk.NewCheckBox(footer)
	if err != nil {
		return err
	}
	if err := ui.openAfter.SetText("Открыть файл после создания"); err != nil {
		return err
	}
	ui.openAfter.SetChecked(cfg.OpenAfterCreate)

	if _, err := walk.NewHSpacer(footer); err != nil {
		return err
	}

	exitBtn, err := walk.NewPushButton(footer)
	if err != nil {
		return err
	}
	if err := exitBtn.SetText("Выход"); err != nil {
		return err
	}
	if err := exitBtn.SetMinMaxSize(walk.Size{Width: 110}, walk.Size{}); err != nil {
		return err
	}
	exitBtn.Clicked().Attach(func() { ui.mw.Close() })

	return nil
}

func (ui *appUI) buildSequenceGroup(parent walk.Container, cfg AppSettings) error {
	gb, err := ui.newGroup(parent, "Последовательность номеров")
	if err != nil {
		return err
	}

	ui.sequence, err = walk.NewLineEdit(gb)
	if err != nil {
		return err
	}
	if err := ui.sequence.SetText(cfg.Sequence); err != nil {
		return err
	}
	ui.sequence.TextChanged().Attach(ui.refresh)

	ui.dedupe, err = walk.NewCheckBox(gb)
	if err != nil {
		return err
	}
	if err := ui.dedupe.SetText("Убрать повторяющиеся номера"); err != nil {
		return err
	}
	ui.dedupe.SetChecked(cfg.RemoveDuplicates)
	ui.dedupe.CheckedChanged().Attach(ui.refresh)

	return nil
}

func (ui *appUI) buildPageGroup(parent walk.Container, cfg AppSettings) error {
	gb, err := ui.newGroup(parent, "Страница")
	if err != nil {
		return err
	}

	presetNames := presetNameList()

	row, _, err := ui.newRow(gb, "Размер:")
	if err != nil {
		return err
	}
	ui.preset, err = walk.NewComboBox(row)
	if err != nil {
		return err
	}
	if err := ui.preset.SetModel(presetNames); err != nil {
		return err
	}
	idx := indexOfString(presetNames, cfg.PagePreset)
	if idx < 0 {
		idx = len(presetNames) - 1
	}
	if err := ui.preset.SetCurrentIndex(idx); err != nil {
		return err
	}
	ui.preset.CurrentIndexChanged().Attach(ui.refresh)

	row, _, err = ui.newRow(gb, "Свой размер, мм:")
	if err != nil {
		return err
	}
	ui.pageW, err = ui.newNumberEdit(row, 20, 2000, cfg.CustomWMM)
	if err != nil {
		return err
	}
	ui.pageW.SetMinMaxSize(walk.Size{Width: 90}, walk.Size{})
	if _, err := walk.NewHSpacerFixed(row, 8); err != nil {
		return err
	}
	ui.pageH, err = ui.newNumberEdit(row, 20, 2000, cfg.CustomHMM)
	if err != nil {
		return err
	}
	ui.pageH.SetMinMaxSize(walk.Size{Width: 90}, walk.Size{})

	ui.landscape, err = walk.NewCheckBox(gb)
	if err != nil {
		return err
	}
	if err := ui.landscape.SetText("Альбомная ориентация"); err != nil {
		return err
	}
	ui.landscape.SetChecked(cfg.Landscape)
	ui.landscape.CheckedChanged().Attach(ui.refresh)

	return nil
}

func (ui *appUI) buildFontGroup(parent walk.Container, cfg AppSettings) error {
	gb, err := ui.newGroup(parent, "Номер")
	if err != nil {
		return err
	}

	row, _, err := ui.newRow(gb, "Шрифт:")
	if err != nil {
		return err
	}
	ui.fontFamily, err = walk.NewComboBox(row)
	if err != nil {
		return err
	}
	if err := ui.fontFamily.SetModel(append([]string{}, FontPresets...)); err != nil {
		return err
	}
	if idx := indexOfString(FontPresets, cfg.FontFamily); idx >= 0 {
		if err := ui.fontFamily.SetCurrentIndex(idx); err != nil {
			return err
		}
	}
	ui.fontFamily.CurrentIndexChanged().Attach(ui.refresh)

	row, _, err = ui.newRow(gb, "Размер, pt:")
	if err != nil {
		return err
	}
	ui.fontSize, err = ui.newNumberEdit(row, 4, 200, cfg.FontSizePt)
	if err != nil {
		return err
	}

	row, _, err = ui.newRow(gb, "Цвет:")
	if err != nil {
		return err
	}
	ui.color, err = walk.NewComboBox(row)
	if err != nil {
		return err
	}
	if err := ui.color.SetModel(ColorLabels()); err != nil {
		return err
	}
	colorIdx := 0
	for i, c := range ColorChoices {
		if c.Name == cfg.Color {
			colorIdx = i
		}
	}
	if err := ui.color.SetCurrentIndex(colorIdx); err != nil {
		return err
	}
	ui.color.CurrentIndexChanged().Attach(ui.refresh)

	row, prefixLabel, err := ui.newRow(gb, "До номера:")
	if err != nil {
		return err
	}
	if err := prefixLabel.SetToolTipText(prefixHint); err != nil {
		return err
	}
	ui.prefix, err = walk.NewLineEdit(row)
	if err != nil {
		return err
	}
	if err := ui.prefix.SetText(cfg.Prefix); err != nil {
		return err
	}
	if err := ui.prefix.SetToolTipText(prefixHint); err != nil {
		return err
	}
	ui.prefix.TextChanged().Attach(ui.refresh)

	row, suffixLabel, err := ui.newRow(gb, "После номера:")
	if err != nil {
		return err
	}
	if err := suffixLabel.SetToolTipText(suffixHint); err != nil {
		return err
	}
	ui.suffix, err = walk.NewLineEdit(row)
	if err != nil {
		return err
	}
	if err := ui.suffix.SetText(cfg.Suffix); err != nil {
		return err
	}
	if err := ui.suffix.SetToolTipText(suffixHint); err != nil {
		return err
	}
	ui.suffix.TextChanged().Attach(ui.refresh)

	fieldsHint, err := walk.NewLabel(gb)
	if err != nil {
		return err
	}
	if err := fieldsHint.SetText("«До номера» и «После номера» печатаются вместе с номером: «Стр. 42», «— 42 —». Пустое поле — на странице только номер."); err != nil {
		return err
	}
	if err := fieldsHint.SetToolTipText(prefixHint + "\n\n" + suffixHint); err != nil {
		return err
	}

	row, _, err = ui.newRow(gb, "Начертание:")
	if err != nil {
		return err
	}
	ui.bold, err = walk.NewCheckBox(row)
	if err != nil {
		return err
	}
	if err := ui.bold.SetText("Жирный"); err != nil {
		return err
	}
	ui.bold.SetChecked(cfg.Bold)
	ui.bold.CheckedChanged().Attach(ui.refresh)

	if _, err := walk.NewHSpacerFixed(row, 12); err != nil {
		return err
	}

	ui.italic, err = walk.NewCheckBox(row)
	if err != nil {
		return err
	}
	if err := ui.italic.SetText("Курсив"); err != nil {
		return err
	}
	ui.italic.SetChecked(cfg.Italic)
	ui.italic.CheckedChanged().Attach(ui.refresh)

	return nil
}

func (ui *appUI) buildPositionGroup(parent walk.Container, cfg AppSettings) error {
	gb, err := ui.newGroup(parent, "Расположение номера")
	if err != nil {
		return err
	}

	row, _, err := ui.newRow(gb, "По горизонтали:")
	if err != nil {
		return err
	}
	ui.colButtons, err = ui.newRadioButtons(row, []string{"слева", "по центру", "справа"}, cfg.Col)
	if err != nil {
		return err
	}

	row, _, err = ui.newRow(gb, "По вертикали:")
	if err != nil {
		return err
	}
	ui.rowButtons, err = ui.newRadioButtons(row, []string{"сверху", "по центру", "снизу"}, cfg.Row)
	if err != nil {
		return err
	}

	row, _, err = ui.newRow(gb, "Отступ слева/справа, мм:")
	if err != nil {
		return err
	}
	ui.marginX, err = ui.newNumberEdit(row, 0, 300, cfg.MarginXMM)
	if err != nil {
		return err
	}

	row, _, err = ui.newRow(gb, "Отступ сверху/снизу, мм:")
	if err != nil {
		return err
	}
	ui.marginY, err = ui.newNumberEdit(row, 0, 300, cfg.MarginYMM)
	if err != nil {
		return err
	}

	return nil
}

func (ui *appUI) buildPreview(parent walk.Container) error {
	iv, err := walk.NewImageView(parent)
	if err != nil {
		return err
	}
	iv.SetMode(walk.ImageViewModeShrink)
	iv.SetMargin(4)
	if err := iv.SetMinMaxSize(walk.Size{Width: 300, Height: 330}, walk.Size{}); err != nil {
		return err
	}
	ui.preview = iv

	note, err := walk.NewLabel(parent)
	if err != nil {
		return err
	}
	if err := note.SetText("Предпросмотр первой страницы — номер показан крупнее, чем будет в PDF"); err != nil {
		return err
	}

	// Область превью получает настоящий размер только после раскладки окна,
	// поэтому перерисовываем её при каждом изменении размеров — иначе
	// картинка, посчитанная по предположительному размеру, потом
	// масштабируется и «уезжает».
	ui.preview.SizeChanged().Attach(func() { ui.updatePreview() })
	ui.mw.SizeChanged().Attach(func() { ui.updatePreview() })
	return nil
}

// ---------- вспомогательные конструкторы ----------

func (ui *appUI) newGroup(parent walk.Container, title string) (*walk.GroupBox, error) {
	gb, err := walk.NewGroupBox(parent)
	if err != nil {
		return nil, err
	}
	if err := gb.SetTitle(title); err != nil {
		return nil, err
	}
	vbox := walk.NewVBoxLayout()
	vbox.SetSpacing(6)
	if err := gb.SetLayout(vbox); err != nil {
		return nil, err
	}
	return gb, nil
}

// newRow создаёт строку «подпись + элемент управления».
func (ui *appUI) newRow(parent walk.Container, label string) (*walk.Composite, *walk.Label, error) {
	row, _, err := ui.newHBox(parent, 6)
	if err != nil {
		return nil, nil, err
	}
	l, err := walk.NewLabel(row)
	if err != nil {
		return nil, nil, err
	}
	if err := l.SetText(label); err != nil {
		return nil, nil, err
	}
	if err := l.SetMinMaxSize(walk.Size{Width: 170}, walk.Size{}); err != nil {
		return nil, nil, err
	}
	return row, l, nil
}

func (ui *appUI) newHBox(parent walk.Container, spacing int) (*walk.Composite, *walk.BoxLayout, error) {
	box, err := walk.NewComposite(parent)
	if err != nil {
		return nil, nil, err
	}
	hb := walk.NewHBoxLayout()
	hb.SetSpacing(spacing)
	if err := box.SetLayout(hb); err != nil {
		return nil, nil, err
	}
	return box, hb, nil
}

func (ui *appUI) newVBox(parent walk.Container, spacing int) (*walk.Composite, error) {
	box, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	vb, err := vboxLayout(0, spacing)
	if err != nil {
		return nil, err
	}
	if err := box.SetLayout(vb); err != nil {
		return nil, err
	}
	return box, nil
}

// vboxLayout создаёт вертикальную раскладку с полями и интервалом.
func vboxLayout(margin, spacing int) (*walk.BoxLayout, error) {
	vb := walk.NewVBoxLayout()
	if margin > 0 {
		if err := vb.SetMargins(walk.Margins{HNear: margin, VNear: margin, HFar: margin, VFar: margin}); err != nil {
			return nil, err
		}
	}
	vb.SetSpacing(spacing)
	return vb, nil
}

func (ui *appUI) newNumberEdit(parent walk.Container, min, max, value float64) (*walk.NumberEdit, error) {
	ne, err := walk.NewNumberEdit(parent)
	if err != nil {
		return nil, err
	}
	if err := ne.SetRange(min, max); err != nil {
		return nil, err
	}
	if err := ne.SetValue(value); err != nil {
		return nil, err
	}
	ne.ValueChanged().Attach(ui.refresh)
	return ne, nil
}

func (ui *appUI) newRadioButtons(parent walk.Container, items []string, selected int) ([]*walk.RadioButton, error) {
	buttons := make([]*walk.RadioButton, 0, len(items))
	for i, item := range items {
		rb, err := walk.NewRadioButton(parent)
		if err != nil {
			return nil, err
		}
		if err := rb.SetText(item); err != nil {
			return nil, err
		}
		if i == selected {
			rb.SetChecked(true)
		}
		rb.Clicked().Attach(ui.refresh)
		buttons = append(buttons, rb)
	}
	return buttons, nil
}

// ---------- параметры ----------

func presetNameList() []string {
	names := make([]string, 0, len(PagePresets)+1)
	for _, p := range PagePresets {
		names = append(names, p.Name)
	}
	return append(names, CustomPageName)
}

func (ui *appUI) pageSizeMM() (w, h float64) {
	names := presetNameList()
	idx := ui.preset.CurrentIndex()
	if idx >= 0 && idx < len(names) && names[idx] != CustomPageName {
		for _, p := range PagePresets {
			if p.Name == names[idx] {
				return p.W, p.H
			}
		}
	}
	return ui.pageW.Value(), ui.pageH.Value()
}

func (ui *appUI) selectedRadio(buttons []*walk.RadioButton, def int) int {
	for i, rb := range buttons {
		if rb.Checked() {
			return i
		}
	}
	return def
}

func (ui *appUI) colorChoice() ColorChoice {
	idx := ui.color.CurrentIndex()
	if idx >= 0 && idx < len(ColorChoices) {
		return ColorChoices[idx]
	}
	return ColorChoices[0]
}

func (ui *appUI) options() (GenerateOptions, error) {
	var o GenerateOptions

	numbers, err := ParseSequence(ui.sequence.Text(), SeqOptions{RemoveDuplicates: ui.dedupe.Checked()})
	if err != nil {
		return o, err
	}

	w, h := ui.pageSizeMM()

	o = GenerateOptions{
		Numbers:    numbers,
		PageWMM:    w,
		PageHMM:    h,
		Landscape:  ui.landscape.Checked(),
		MarginXMM:  ui.marginX.Value(),
		MarginYMM:  ui.marginY.Value(),
		Row:        ui.selectedRadio(ui.rowButtons, RowBottom),
		Col:        ui.selectedRadio(ui.colButtons, ColCenter),
		FontFamily: ui.fontFamily.Text(),
		FontSizePt: ui.fontSize.Value(),
		Bold:       ui.bold.Checked(),
		Italic:     ui.italic.Checked(),
		Color:      ui.colorChoice().RGB,
		Prefix:     ui.prefix.Text(),
		Suffix:     ui.suffix.Text(),
		Title:      "Страницы с номерами",
	}

	if err := o.Validate(); err != nil {
		return o, err
	}
	return o, nil
}

func (ui *appUI) settings() AppSettings {
	s := DefaultSettings()

	s.Sequence = ui.sequence.Text()
	s.RemoveDuplicates = ui.dedupe.Checked()
	names := presetNameList()
	if idx := ui.preset.CurrentIndex(); idx >= 0 && idx < len(names) {
		s.PagePreset = names[idx]
	}
	s.CustomWMM = ui.pageW.Value()
	s.CustomHMM = ui.pageH.Value()
	s.Landscape = ui.landscape.Checked()

	s.MarginXMM = ui.marginX.Value()
	s.MarginYMM = ui.marginY.Value()
	s.Row = ui.selectedRadio(ui.rowButtons, RowBottom)
	s.Col = ui.selectedRadio(ui.colButtons, ColCenter)

	s.FontFamily = ui.fontFamily.Text()
	s.FontSizePt = ui.fontSize.Value()
	s.Bold = ui.bold.Checked()
	s.Italic = ui.italic.Checked()
	s.Color = ui.colorChoice().Name
	s.Prefix = ui.prefix.Text()
	s.Suffix = ui.suffix.Text()
	s.OpenAfterCreate = ui.openAfter.Checked()
	s.LastDir = ui.lastDir

	return s
}

// ---------- обновление состояния ----------

func (ui *appUI) refresh() {
	custom := ui.preset.CurrentIndex() == len(PagePresets) // последний пункт — "Свой"
	ui.pageW.SetEnabled(custom)
	ui.pageH.SetEnabled(custom)

	opts, err := ui.options()
	if err != nil {
		ui.status.SetText("✖ " + err.Error())
		ui.preview.SetImage(nil)
		return
	}

	ui.status.SetText(fmt.Sprintf("Страниц: %d — %s", len(opts.Numbers), shortRange(opts.Numbers)))
	ui.updatePreviewWith(opts)
}

func (ui *appUI) updatePreview() {
	if ui.preview == nil {
		return
	}
	opts, err := ui.options()
	if err != nil {
		return
	}
	ui.updatePreviewWith(opts)
}

func (ui *appUI) updatePreviewWith(opts GenerateOptions) {
	if ui.preview == nil || len(opts.Numbers) == 0 {
		return
	}

	size := ui.preview.Size()
	w, h := size.Width, size.Height
	if w < 40 || h < 40 {
		// размер ещё неизвестен — рисуем в типовом поле и перерисуем позже
		w, h = 300, 330
	}
	if w == ui.previewW && h == ui.previewH && ui.previewOpts == optsKey(opts) {
		return // картинка актуальна
	}
	ui.previewW, ui.previewH, ui.previewOpts = w, h, optsKey(opts)

	img := RenderPreview(opts, opts.Numbers[0], w, h)
	if img == nil {
		ui.preview.SetImage(nil)
		return
	}
	bm, err := walk.NewBitmapFromImage(img)
	if err != nil {
		return
	}
	ui.preview.SetImage(bm)
}

// ---------- создание файла ----------

func (ui *appUI) onCreateClicked() {
	opts, err := ui.options()
	if err != nil {
		walk.MsgBox(ui.mw, appTitle, err.Error(), walk.MsgBoxIconError)
		return
	}

	dir := ui.lastDir
	if dir == "" {
		if d, err := os.Getwd(); err == nil {
			dir = d
		}
	}

	dlg := walk.FileDialog{
		Title:          "Сохранить PDF",
		Filter:         "PDF-файлы (*.pdf)|*.pdf|Все файлы (*.*)|*.*",
		InitialDirPath: dir,
		FilePath:       filepath.Join(dir, "pages.pdf"),
	}
	accepted, err := dlg.ShowSave(ui.mw)
	if err != nil {
		walk.MsgBox(ui.mw, appTitle, err.Error(), walk.MsgBoxIconError)
		return
	}
	if !accepted {
		return
	}

	path := dlg.FilePath
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}

	if err := writePDF(path, opts); err != nil {
		walk.MsgBox(ui.mw, appTitle, "Не удалось создать файл:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}

	ui.lastDir = filepath.Dir(path)
	if err := SaveSettings(ui.settings()); err != nil {
		walk.MsgBox(ui.mw, appTitle, "Файл создан, но настройки не сохранены:\n"+err.Error(), walk.MsgBoxIconWarning)
	}

	ui.status.SetText(fmt.Sprintf("✔ Создано страниц: %d — %s", len(opts.Numbers), path))

	if ui.openAfter.Checked() {
		openInViewer(path)
	}
}

// writePDF пишет PDF через временный файл, чтобы не оставлять повреждённых файлов.
func writePDF(path string, opts GenerateOptions) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "blank-num-gen-*.pdf")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := GeneratePDF(tmp, opts); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpName, path)
}

func openInViewer(path string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	if err := cmd.Start(); err != nil {
		walk.MsgBox(nil, appTitle, "Не удалось открыть файл:\n"+err.Error(), walk.MsgBoxIconWarning)
	}
}

func indexOfString(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// optsKey — короткий ключ параметров, влияющих на миниатюру.
func optsKey(o GenerateOptions) string {
	return fmt.Sprintf("%v|%g|%g|%t|%g|%g|%g|%d|%d|%s|%g|%t|%t|%d,%d,%d|%s|%s",
		o.Numbers, o.PageWMM, o.PageHMM, o.Landscape, o.MarginXMM, o.MarginYMM, o.LineHeightMM(),
		o.Row, o.Col, o.FontFamily, o.FontSizePt, o.Bold, o.Italic,
		o.Color.R, o.Color.G, o.Color.B, o.Prefix, o.Suffix)
}
