package main

import _ "embed"

// DejaVu Sans (лицензия Bitstream Vera / DejaVu, см. assets/fonts/LICENSE-DejaVu.txt).
// Шрифты встроены в приложение: они используются, когда в префиксе или
// суффиксе есть кириллица (стандартные шрифты PDF её не поддерживают),
// а также при отрисовке предпросмотра.

//go:embed assets/fonts/DejaVuSans.ttf
var dejavuSans []byte

//go:embed assets/fonts/DejaVuSans-Bold.ttf
var dejavuSansBold []byte

//go:embed assets/fonts/DejaVuSans-Oblique.ttf
var dejavuSansOblique []byte

//go:embed assets/fonts/DejaVuSans-BoldOblique.ttf
var dejavuSansBoldOblique []byte
