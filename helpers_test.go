package gojob_test

import "github.com/WangYihang/gojob"

// collect drains a result stream into a slice.
func collect[T any](in <-chan gojob.Result[T]) []gojob.Result[T] {
	var out []gojob.Result[T]
	for r := range in {
		out = append(out, r)
	}
	return out
}

// rangeInts returns []int{0, 1, ..., n-1}.
func rangeInts(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}
