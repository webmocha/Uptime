
build:
	go build -o bin/uptime .

build-linux64:
	env GOOS=linux GOARCH=amd64 go build -o bin/webhook .

run:
	./bin/uptime

dev: build run

