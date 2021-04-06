// A test
package main

var locals []string

func main() {
	locals = append(locals, "xyz")
	locals = append(locals, "foo")
	locals = append(locals, "billy")
	locals = append(locals, "Hello world")
	locals = append(locals, "")
	locals = append(locals, "...")
	i := 0
	s := ""
	for i < 6 {
		print(locals[i] + "\n")
		i = i + 1
	}
}
