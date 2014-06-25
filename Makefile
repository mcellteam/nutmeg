

all: ctags
	go build -o nutmeg *.go

ctags: tags
	ctags *.go
