package colorollout

import (
	"context"
	"testing"
	"time"
)

func TestQueryLimitAIMD(t *testing.T) {
	q := NewQueryLimit(2, 10)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		q.Acquire(ctx)
		q.Release(true)
	}
	if q.Limit() != 10 {
		t.Fatalf("after successes: %v, want the max", q.Limit())
	}
	q.Acquire(ctx)
	q.Release(false)
	if q.Limit() != 5 {
		t.Fatalf("after a failure: %v", q.Limit())
	}
	for i := 0; i < 10; i++ {
		q.Acquire(ctx)
		q.Release(false)
	}
	if q.Limit() != 2 {
		t.Fatalf("after many failures: %v, want the floor", q.Limit())
	}
}

func TestQueryLimitBlocks(t *testing.T) {
	q := NewQueryLimit(1, 1)
	q.Acquire(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := q.Acquire(ctx); err == nil {
		t.Fatal("second query let through over the limit")
	}
	done := make(chan error)
	go func() { done <- q.Acquire(context.Background()) }()
	q.Release(true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
