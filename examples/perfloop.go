package main

var (
	sum int // global so Go doesn't optimize it out
)

func main() {
	i := 0
	for i < 1000000000 {
		sum = sum + i
		i = i + 1
	}
}
