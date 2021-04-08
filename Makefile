
.PHONY: test
test:
	go run . <examples/test.go >build/test.asm
	nasm -felf64 -F stabs -o build/test.o build/test.asm
	ld -o build/test build/test.o

.PHONY: mugo
mugo:
	go build -o build/mugo

.PHONY: mugo2
mugo2:
	build/mugo <mugo.go >build/mugo2.asm
	nasm -felf64 -F stabs -o build/mugo2.o build/mugo2.asm
	ld -o build/mugo2 build/mugo2.o

.PHONY: mugo3
mugo3:
	build/mugo2 <mugo.go >build/mugo3.asm
	nasm -felf64 -F stabs -o build/mugo3.o build/mugo3.asm
	ld -o build/mugo3 build/mugo3.o
	diff build/mugo2.asm build/mugo3.asm

.PHONY: coverage
coverage:
	go test -c -o build/mugo_test -cover
	build/mugo_test -test.coverprofile build/coverage.out <mugo.go >/dev/null
	go tool cover -html build/coverage.out -o build/coverage.html
