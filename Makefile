build:
	go build -o bin/http-send ./cmd/http_send/main.go

http-bin: build
	./bin/http-send http://httpbin.org/get
