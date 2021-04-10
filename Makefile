
.PHONY: hello
.PHONY: mugo
.PHONY: mugo2
.PHONY: mugo3
.PHONY: coverage
.PHONY: perfloop
.PHONY: all

hello:
	go run . <examples/hello.go >build/hello.asm
	nasm -felf64 -o build/hello.o build/hello.asm
	ld -o build/hello build/hello.o

# Build the compiler with Go
mugo:
	go build -o build/mugo

# Build the compiler with the Go-built Mugo
mugo2:
	build/mugo <mugo.go >build/mugo2.asm
	nasm -felf64 -o build/mugo2.o build/mugo2.asm
	ld -o build/mugo2 build/mugo2.o

# Build the compiler with the Mugo-built Mugo
mugo3:
	build/mugo2 <mugo.go >build/mugo3.asm
	nasm -felf64 -o build/mugo3.o build/mugo3.asm
	ld -o build/mugo3 build/mugo3.o
	diff build/mugo2.asm build/mugo3.asm

coverage:
	go test -c -o build/mugo_test -cover
	build/mugo_test -test.coverprofile build/coverage.out <mugo.go >/dev/null
	go tool cover -html build/coverage.out -o coverage.html

perfloop:
	go build -o build/perfloop-go examples/perfloop.go
	go run . <examples/perfloop.go >build/perfloop.asm
	nasm -felf64 -o build/perfloop.o build/perfloop.asm
	ld -o build/perfloop-mugo build/perfloop.o


all: hello mugo mugo2 mugo3 coverage perfloop
