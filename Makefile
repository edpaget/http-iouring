build:
	go build -o bin/http-send ./cmd/http_send/main.go
	go build -o bin/uring-cat ./cmd/uring_cat/main.go

http-bin: build
	./bin/http-send http://httpbin.org/get

docker-build:
	docker build -t edpaget/ioring .

docker-bash:
	docker run -v "$(PWD):/app" -i -t edpaget/ioring /bin/bash
