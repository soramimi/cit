.PHONY: build run test clean

run:
	go run .

build:
	go build -o cit

test:
	go test ./...

clean:
	rm -f appgo get github.com/rivo/tview

setup:
	go get github.com/rivo/tview

install:
	install -m 755 cit ~/.local/bin/
