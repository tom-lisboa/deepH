package runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestToolBrokerCachesSequentialCalls(t *testing.T) {
	b := newToolBroker()
	var runs atomic.Int32

	fn := func(ctx context.Context) (map[string]any, error) {
		_ = ctx
		runs.Add(1)
		return map[string]any{"v": "ok"}, nil
	}

	out1, err1, hit1 := b.Do(context.Background(), "k1", fn)
	out2, err2, hit2 := b.Do(context.Background(), "k1", fn)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: err1=%v err2=%v", err1, err2)
	}
	if hit1 {
		t.Fatalf("first call should be a miss")
	}
	if !hit2 {
		t.Fatalf("second call should be a cache hit")
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("expected fn to execute once, got %d", got)
	}
	out1["mutated"] = true
	if _, ok := out2["mutated"]; ok {
		t.Fatalf("expected cloned map results, second result mutated by first")
	}
}

func TestToolBrokerCoalescesConcurrentCalls(t *testing.T) {
	b := newToolBroker()
	var runs atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	fn := func(ctx context.Context) (map[string]any, error) {
		runs.Add(1)
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return map[string]any{"v": "ok"}, nil
	}

	type res struct {
		hit bool
		err error
	}
	var wg sync.WaitGroup
	got := make(chan res, 2)
	call := func() {
		defer wg.Done()
		_, err, hit := b.Do(context.Background(), "same", fn)
		got <- res{hit: hit, err: err}
	}
	wg.Add(2)
	go call()
	<-started
	go call()
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	close(got)

	var hits, misses int
	for r := range got {
		if r.err != nil {
			t.Fatalf("unexpected error: %v", r.err)
		}
		if r.hit {
			hits++
		} else {
			misses++
		}
	}
	if misses != 1 || hits != 1 {
		t.Fatalf("expected 1 miss + 1 coalesced hit, got misses=%d hits=%d", misses, hits)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("expected underlying fn once, got %d", got)
	}
}

func TestToolBrokerResourceLockSerializesDifferentKeysSameResource(t *testing.T) {
	b := newToolBroker()
	var inFlight atomic.Int32
	var maxSeen atomic.Int32
	startedFirst := make(chan struct{})
	releaseFirst := make(chan struct{})

	run := func(label string, release <-chan struct{}, started chan<- struct{}) {
		t.Helper()
		_, err := b.WithResourceLock(context.Background(), "file:README.md", func(ctx context.Context) (map[string]any, error) {
			_ = ctx
			cur := inFlight.Add(1)
			for {
				prev := maxSeen.Load()
				if cur <= prev || maxSeen.CompareAndSwap(prev, cur) {
					break
				}
			}
			if started != nil {
				close(started)
			}
			if release != nil {
				<-release
			}
			inFlight.Add(-1)
			return map[string]any{"label": label}, nil
		})
		if err != nil {
			t.Fatalf("%s lock execution err: %v", label, err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		run("first", releaseFirst, startedFirst)
	}()
	<-startedFirst
	go func() {
		defer wg.Done()
		run("second", nil, nil)
	}()
	time.Sleep(20 * time.Millisecond)
	close(releaseFirst)
	wg.Wait()

	if got := maxSeen.Load(); got != 1 {
		t.Fatalf("expected serialized resource lock max concurrency=1, got %d", got)
	}
}
