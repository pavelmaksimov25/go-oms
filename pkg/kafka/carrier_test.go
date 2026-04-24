package kafka

import (
	"slices"
	"testing"
)

func TestHeaderCarrier_SetAndGet(t *testing.T) {
	var c headerCarrier
	c.Set("traceparent", "abc")

	if got := c.Get("traceparent"); got != "abc" {
		t.Errorf("Get = %q, want abc", got)
	}
}

func TestHeaderCarrier_SetOverwrites(t *testing.T) {
	var c headerCarrier
	c.Set("key", "first")
	c.Set("key", "second")

	if got := c.Get("key"); got != "second" {
		t.Errorf("Get = %q, want second", got)
	}
	if len(c) != 1 {
		t.Errorf("header count = %d, want 1 (Set should overwrite)", len(c))
	}
}

func TestHeaderCarrier_Keys(t *testing.T) {
	var c headerCarrier
	c.Set("a", "1")
	c.Set("b", "2")

	got := c.Keys()
	slices.Sort(got)
	want := []string{"a", "b"}
	if !slices.Equal(got, want) {
		t.Errorf("Keys = %v, want %v", got, want)
	}
}

func TestHeaderCarrier_GetMissing_ReturnsEmpty(t *testing.T) {
	var c headerCarrier
	if got := c.Get("nope"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}
}
