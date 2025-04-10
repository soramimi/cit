.PHONY: build run test clean

run:
	go run .

build:
	go build -o app

test:
	go test ./...

clean:
	rm -f appgo get github.com/rivo/tview

setup:
	go get github.com/rivo/tview
