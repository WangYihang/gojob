package utils_test

// genInts returns a bidirectional channel that emits 0..n-1 and then closes.
// It is bidirectional so it can be passed to helpers that take chan T as well
// as those that take <-chan T.
func genInts(n int) chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := 0; i < n; i++ {
			out <- i
		}
	}()
	return out
}

// collectInts drains a channel into a slice.
func collectInts(in <-chan int) []int {
	out := []int{}
	for v := range in {
		out = append(out, v)
	}
	return out
}
