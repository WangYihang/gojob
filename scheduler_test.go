package gojob_test

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/WangYihang/gojob"
)

type SafeWriter struct {
	writer *strings.Builder
	lock   sync.Mutex
}

func NewSafeWriter() *SafeWriter {
	return &SafeWriter{
		writer: new(strings.Builder),
		lock:   sync.Mutex{},
	}
}

func (sw *SafeWriter) WriteString(s string) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	sw.writer.WriteString(s)
}

func (sw *SafeWriter) String() string {
	return sw.writer.String()
}

type Task struct {
	I      int
	writer *SafeWriter
}

func NewTask(i int, writer *SafeWriter) *Task {
	return &Task{
		I:      i,
		writer: writer,
	}
}

func (t *Task) Do() error {
	t.writer.WriteString(fmt.Sprintf("%d\n", t.I))
	return nil
}

func TestSharding(t *testing.T) {
	testcases := []struct {
		numShards int64
		shard     int64
		expected  []int
	}{
		{
			numShards: 2,
			shard:     0,
			expected:  []int{0, 2, 4, 6, 8, 10, 12, 14},
		},
		{
			numShards: 2,
			shard:     1,
			expected:  []int{1, 3, 5, 7, 9, 11, 13, 15},
		},
		{
			numShards: 3,
			shard:     0,
			expected:  []int{0, 3, 6, 9, 12, 15},
		},
		{
			numShards: 3,
			shard:     1,
			expected:  []int{1, 4, 7, 10, 13},
		},
		{
			numShards: 3,
			shard:     2,
			expected:  []int{2, 5, 8, 11, 14},
		},
	}
	for _, tc := range testcases {
		safeWriter := NewSafeWriter()
		scheduler := gojob.NewScheduler().SetNumShards(tc.numShards).SetShard(tc.shard).SetOutputFilePath("").Start()
		for i := 0; i < 16; i++ {
			scheduler.Submit(NewTask(i, safeWriter))
		}
		scheduler.Wait()
		output := safeWriter.String()
		lines := strings.Split(output, "\n")
		numbers := []int{}
		for _, line := range lines {
			if line == "" {
				continue
			}
			number, err := strconv.Atoi(line)
			if err != nil {
				t.Fatal(err)
			}
			numbers = append(numbers, number)
		}
		sort.Ints(numbers)
		if !reflect.DeepEqual(numbers, tc.expected) {
			t.Errorf("Expected %v, got %v", tc.expected, numbers)
		}
	}
}
