// A test
package main

func intStr(n int) string {
	if n < 0 {
		return "-" + intStr(-n)
	}
	if n < 10 {
		return charStr(n + '0')
	}
	return intStr(n/10) + intStr(n%10)
}

var (
	c int
	line int
	col int
)

func nextChar() {
	c = readByte()
	col = col + 1
	if c == '\n' {
		line = line + 1
		col = 0
	}
}

func main() {
	nextChar()
}
