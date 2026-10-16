//go:build !windows

package main

import "fmt"

// runGUI доступен только в Windows-сборке. Соберите приложение под Windows
// (GOOS=windows GOARCH=amd64 go build) или используйте консольный режим:
//
//	blank_num_gen -o out.pdf -s "1,2,6-9,42-43"
func runGUI() error {
	return fmt.Errorf("графический интерфейс доступен только в Windows-сборке; используйте консольный режим (см. -h)")
}

func showFatalError(err error) {
	fmt.Println("Ошибка:", err)
}
