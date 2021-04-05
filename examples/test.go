// A test
package main

// 0   8   16
// rbp ret n
func t(n int) string {
	i := 0
	s := ""
// label1:
	for i < n {
		s = s + "x"
		i = i + 1
	}
	return s
}

func main() {
	print(t(80) + "\n")
}
