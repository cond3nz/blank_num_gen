# BlankNumGen — сборка
#
#   make windows   — статические .exe для Windows (64 и 32 бита)
#   make cli       — консольная сборка (та же логика, но с выводом в терминал)
#   make test      — тесты
#   make manifest  — пересобрать .syso из app.manifest и app.ico (нужна сеть)
#   make icon      — перерисовать app.ico

APP      := blank_num_gen
DIST     := dist
LDFLAGS  := -s -w
GOFLAGS  := -trimpath
RSRC     := github.com/akavel/rsrc@latest

.PHONY: all windows windows-amd64 windows-386 cli test vet fmt icon manifest clean

all: windows

windows: windows-amd64 windows-386

windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS) -H=windowsgui" -o $(DIST)/$(APP).exe .
	@echo "==> $(DIST)/$(APP).exe"

windows-386:
	GOOS=windows GOARCH=386 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS) -H=windowsgui" -o $(DIST)/$(APP)-32.exe .
	@echo "==> $(DIST)/$(APP)-32.exe"

# Консольная версия: запускается из cmd/PowerShell и печатает результат
cli: windows-amd64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP)-cli.exe .
	@echo "==> $(DIST)/$(APP)-cli.exe"

test:
	go test ./...

vet:
	go vet ./...
	GOOS=windows GOARCH=amd64 go vet ./...

fmt:
	gofmt -l -w .

icon:
	go run ./tools/mkicon -out app.ico

manifest:
	go run $(RSRC) -manifest app.manifest -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso
	go run $(RSRC) -manifest app.manifest -ico app.ico -arch 386   -o rsrc_windows_386.syso

clean:
	rm -rf $(DIST)