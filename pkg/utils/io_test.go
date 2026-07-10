package utils_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

func TestHead(t *testing.T) {
	got := collectInts(utils.Head(genInts(10), 3))
	want := []int{0, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Head: want %v, got %v", want, got)
	}
}

func TestTail(t *testing.T) {
	got := collectInts(utils.Tail(genInts(10), 3))
	want := []int{7, 8, 9}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tail: want %v, got %v", want, got)
	}
}

func TestSkip(t *testing.T) {
	got := collectInts(utils.Skip(genInts(10), 7))
	want := []int{7, 8, 9}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Skip: want %v, got %v", want, got)
	}
}

func TestCount(t *testing.T) {
	if n := utils.Count(genInts(10)); n != 10 {
		t.Errorf("Count: want 10, got %d", n)
	}
}

func TestCat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("  a  \nb\n  c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for line := range utils.Cat(path) {
		got = append(got, line)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Cat: want %v, got %v", want, got)
	}
}
