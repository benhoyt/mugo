// A test
package main

// func foo(a int, b int) {
// 	// 0   8   16 24
// 	// rbp res b  a
// }

// func intStr(n int) string {
// 	if n < 0 {
// 		return "-" + intStr(-n)
// 	}
// 	if n == 0 {
// 		return "0"
// 	}
// 	if n == 1 {
// 		return "1"
// 	}
// 	if n == 2 {
// 		return "2"
// 	}
// 	if n == 3 {
// 		return "3"
// 	}
// 	if n == 4 {
// 		return "4"
// 	}
// 	if n == 5 {
// 		return "5"
// 	}
// 	if n == 6 {
// 		return "6"
// 	}
// 	if n == 7 {
// 		return "7"
// 	}
// 	if n == 8 {
// 		return "8"
// 	}
// 	if n == 9 {
// 		return "9"
// 	}
// 	return intStr(n/10) + intStr(n%10)
// }

func f() string {
	return "foo" + "bar\n"
}

func main() {
	// print(intStr(1*2 + 3*4))
	print(f())
}
