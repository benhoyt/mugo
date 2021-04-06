// A test
package main

var locals []int

func main() {
	locals = append(locals, 65)
	locals = append(locals, 66)
	locals = append(locals, 67)
	locals = append(locals, 68)
	locals = append(locals, 69)
	i := 0
	s := ""
	for i < 5 {
		s = s + charStr(locals[i])
		i = i + 1
	}
	print(s + "\n")
}
