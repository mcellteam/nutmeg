

all:  ctags
	go build -o nutmeg *.go

ctags: tags
	/usr/local/bin/ctags *.go
