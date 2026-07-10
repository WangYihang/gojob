package utils_test

import (
	"reflect"
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

func TestMap(t *testing.T) {
	out := utils.Map(genInts(5), func(x int) int { return x * x })
	got := collectInts(out)
	want := []int{0, 1, 4, 9, 16}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Map: want %v, got %v", want, got)
	}
}

func TestReduce(t *testing.T) {
	sum := utils.Reduce(genInts(5), func(a, b int) int { return a + b })
	if sum != 10 {
		t.Errorf("Reduce: want %d, got %d", 10, sum)
	}
}
