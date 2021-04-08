package main

import (
	"bufio"
	"os"
)

var stdin = bufio.NewReader(os.Stdin)

func getc() int {
	b, err := stdin.ReadByte()
	if err != nil {
		return -1
	}
	return int(b)
}

func log(s string) {
	os.Stderr.WriteString(s)
}

func print(s string) {
	os.Stdout.WriteString(s)
}

func exit(code int) {
	os.Exit(code)
}

func char(ch int) string {
	return string([]byte{byte(ch)})
}
