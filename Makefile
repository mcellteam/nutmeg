

host:
	go build -o nutmeg *.go


.PHONY: windows_386 windows_amd64 linux_386 linux_amd64 osx_386 osx_amd64 \
	osx linux windows

windows: windows_386 windows_amd64

linux: linux_386 linux_amd64

osx: osx_386 osx_amd64

all: windows linux osx

windows_386:
	GOOS=windows GOARCH=386 go build -o nutmeg_windows_386 *.go

windows_amd64:
	GOOS=windows GOARCH=amd64 go build -o nutmeg_windows_amd64 *.go

linux_386:
	GOOS=linux GOARCH=386 go build -o nutmeg_linux_386 *.go

linux_amd64:
	GOOS=linux GOARCH=amd64 go build -o nutmeg_linux_amd64 *.go

osx_386:
	GOOS=darwin GOARCH=386 go build -o nutmeg_osx_386 *.go

osx_amd64:
	GOOS=darwin GOARCH=amd64 go build -o nutmeg_osx_amd64 *.go

ctags: tags
	ctags *.go

.PHONY: clean

clean:
	rm -f nutmeg_* nutmeg
