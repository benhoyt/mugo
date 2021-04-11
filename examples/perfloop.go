package main

var (
	result int
)

func main() {
	sum := 0
	i := 0
	for i < 1000000000 {
		sum = sum + i
		i = i + 1
	}
	result = sum // so Go doesn't optimize it out
}
