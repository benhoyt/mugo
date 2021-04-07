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

var s string
var sl []int

func main() {
	s = "foo"
	print(intStr(len(s)) + "\n")
	print(intStr(len("foobar")) + "\n")
	print("\n")
	print(intStr(len(sl)) + "\n")
	sl = append(sl, 1)
	sl = append(sl, 2)
	sl = append(sl, 3)
	sl = append(sl, 4)
	print(intStr(len(sl)) + "\n")
	sl = sl[:2]
	print(intStr(len(sl)) + "\n")
	sl = sl[:0]
	print(intStr(len(sl)) + "\n")
}
