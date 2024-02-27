package util

import "sync"

// Fanin takes a slice of channels and returns a single channel that
func Fanin[T interface{}](cs []chan T) chan T {
	var wg sync.WaitGroup
	out := make(chan T)
	output := func(c chan T) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// Fanout takes a channel and returns a slice of channels
// the item in the input channel will be distributed to the output channels
func Fanout[T interface{}](in chan *T, n int) []chan *T {
	cs := make([]chan *T, n)
	for i := 0; i < n; i++ {
		cs[i] = make(chan *T)
		go func(c chan *T) {
			for n := range in {
				c <- n
			}
			close(c)
		}(cs[i])
	}
	return cs
}

// Filter takes a channel and returns a channel with the items that pass the filter
func Filter[T interface{}](in chan T, f func(T) bool) chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for line := range in {
			if f(line) {
				out <- line
			}
		}
	}()
	return out
}
