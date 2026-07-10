package gojob_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/WangYihang/gojob"
)

func TestResultMarshalJSON(t *testing.T) {
	ok := gojob.Result[string]{Value: "v", Attempts: 2}
	b, err := json.Marshal(ok)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["value"] != "v" || m["error"] != "" || m["attempts"].(float64) != 2 {
		t.Errorf("unexpected JSON: %s", b)
	}

	failed := gojob.Result[int]{Err: errors.New("boom"), Attempts: 1}
	b, _ = json.Marshal(failed)
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["error"] != "boom" {
		t.Errorf("expected error \"boom\", got %v", m["error"])
	}
}

func TestTaskFunc(t *testing.T) {
	var task gojob.Task[int] = gojob.TaskFunc[int](func(context.Context) (int, error) {
		return 42, nil
	})
	v, err := task.Execute(context.Background())
	if err != nil || v != 42 {
		t.Errorf("TaskFunc: want 42/nil, got %d/%v", v, err)
	}
}
