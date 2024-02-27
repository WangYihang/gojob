package util

// Map takes a channel and returns a channel with the items that pass the filter
func Map[T interface{}, U interface{}](in chan T, f func(T) U) chan U {
	out := make(chan U)
	go func() {
		defer close(out)
		for line := range in {
			out <- f(line)
		}
	}()
	return out
}

// Reduce takes a channel and returns a channel with the items that pass the filter
func Reduce[T interface{}](in chan T, f func(T, T) T) T {
	var result T
	for line := range in {
		result = f(result, line)
	}
	return result
}
