package main

import (
	"bufio"
	"os"
	"strconv"
)

var stdin = bufio.NewReader(os.Stdin)

func readByte() int {
	b, err := stdin.ReadByte()
	if err != nil {
		return -1
	}
	return int(b)
}

func printError(s string) {
	os.Stderr.WriteString(s)
}

func print(s string) {
	os.Stdout.WriteString(s)
}

func exit(code int) {
	os.Exit(code)
}

// TODO: implement in main.go later
func intStr(n int) string {
	return strconv.Itoa(n)
}

// TODO: implement in main.go later
func charStr(ch int) string {
	return string([]byte{byte(ch)})
}
