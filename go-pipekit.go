package pipekit

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
func Fanout[T interface{}](in chan T, n int) []chan T {
	cs := make([]chan T, n)
	for i := 0; i < n; i++ {
		cs[i] = make(chan T)
		go func(c chan T) {
			for n := range in {
				c <- n
			}
			close(c)
		}(cs[i])
	}
	return cs
}

// Head takes a channel and returns a channel with the first n items
func Head[T interface{}](in chan T, max int) chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		i := 0
		for line := range in {
			if i >= max {
				break
			}
			out <- line
			i++
		}
	}()
	return out
}

// Tail takes a channel and returns a channel with the last n items
func Tail[T interface{}](in chan T, max int) chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		var lines []T
		for line := range in {
			lines = append(lines, line)
			if len(lines) > max {
				lines = lines[1:]
			}
		}
		for _, line := range lines {
			out <- line
		}
	}()
	return out
}
