package tools

import "testing"

func TestResultStore_StoreAndGet(t *testing.T) {
	t.Parallel()

	s := NewResultStore()
	s.Store("call_1", "hello world")

	got, ok := s.Get("call_1")
	if !ok {
		t.Fatal("expected result to be found")
	}
	if got != "hello world" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestResultStore_MissingKey(t *testing.T) {
	t.Parallel()

	s := NewResultStore()
	_, ok := s.Get("no_such_id")
	if ok {
		t.Fatal("expected miss for unknown key")
	}
}
