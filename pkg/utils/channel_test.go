package utils_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

func TestFilter(t *testing.T) {
	out := utils.Filter(genInts(10), func(x int) bool { return x%2 == 0 })
	got := collectInts(out)
	want := []int{0, 2, 4, 6, 8}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter: want %v, got %v", want, got)
	}
}

func TestFanin(t *testing.T) {
	cs := []chan int{make(chan int), make(chan int)}
	go func() {
		defer close(cs[0])
		cs[0] <- 1
		cs[0] <- 3
	}()
	go func() {
		defer close(cs[1])
		cs[1] <- 2
		cs[1] <- 4
	}()
	sum, count := 0, 0
	for v := range utils.Fanin(cs) {
		sum += v
		count++
	}
	if count != 4 || sum != 10 {
		t.Errorf("Fanin: want count=4 sum=10, got count=%d sum=%d", count, sum)
	}
}

func TestFanout(t *testing.T) {
	in := make(chan *int)
	go func() {
		defer close(in)
		for i := 0; i < 10; i++ {
			v := i
			in <- &v
		}
	}()
	outs := utils.Fanout(in, 3)
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		sum   int
		count int
	)
	for _, c := range outs {
		wg.Add(1)
		go func(c chan *int) {
			defer wg.Done()
			for v := range c {
				mu.Lock()
				sum += *v
				count++
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()
	if count != 10 || sum != 45 {
		t.Errorf("Fanout: want count=10 sum=45, got count=%d sum=%d", count, sum)
	}
}
