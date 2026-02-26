package runtime

import (
	"context"
	"sync"
	"time"
)

type toolBroker struct {
	mu       sync.Mutex
	cache    map[string]toolBrokerEntry
	inflight map[string]*toolBrokerInflight
	locks    map[string]*toolBrokerKeyLock
}

type toolBrokerEntry struct {
	result   map[string]any
	errText  string
	cachedAt time.Time
}

type toolBrokerInflight struct {
	done   chan struct{}
	result map[string]any
	err    error
}

type toolBrokerKeyLock struct {
	ch chan struct{}
}

func newToolBroker() *toolBroker {
	return &toolBroker{
		cache:    map[string]toolBrokerEntry{},
		inflight: map[string]*toolBrokerInflight{},
		locks:    map[string]*toolBrokerKeyLock{},
	}
}

// Do deduplicates cacheable tool calls across the current execution.
// It returns cacheHit=true when the result came from cache or a coalesced in-flight call.
func (b *toolBroker) Do(ctx context.Context, key string, fn func(context.Context) (map[string]any, error)) (result map[string]any, err error, cacheHit bool) {
	if b == nil || key == "" {
		r, e := fn(ctx)
		return cloneTopMap(r), e, false
	}

	b.mu.Lock()
	if entry, ok := b.cache[key]; ok {
		b.mu.Unlock()
		if entry.errText != "" {
			return nil, errorString(entry.errText), true
		}
		return cloneTopMap(entry.result), nil, true
	}
	if inf, ok := b.inflight[key]; ok {
		b.mu.Unlock()
		select {
		case <-inf.done:
			if inf.err != nil {
				return nil, inf.err, true
			}
			return cloneTopMap(inf.result), nil, true
		case <-ctx.Done():
			return nil, ctx.Err(), false
		}
	}
	inf := &toolBrokerInflight{done: make(chan struct{})}
	b.inflight[key] = inf
	b.mu.Unlock()

	r, e := fn(ctx)

	b.mu.Lock()
	delete(b.inflight, key)
	inf.result = cloneTopMap(r)
	inf.err = e
	if e == nil {
		b.cache[key] = toolBrokerEntry{
			result:   cloneTopMap(r),
			cachedAt: time.Now(),
		}
	}
	close(inf.done)
	b.mu.Unlock()
	return cloneTopMap(r), e, false
}

func (b *toolBroker) WithResourceLock(ctx context.Context, key string, fn func(context.Context) (map[string]any, error)) (map[string]any, error) {
	if b == nil || key == "" {
		return fn(ctx)
	}
	lk := b.resourceLock(key)
	select {
	case <-lk.ch:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() {
		lk.ch <- struct{}{}
	}()
	return fn(ctx)
}

func (b *toolBroker) resourceLock(key string) *toolBrokerKeyLock {
	b.mu.Lock()
	defer b.mu.Unlock()
	if lk, ok := b.locks[key]; ok {
		return lk
	}
	lk := &toolBrokerKeyLock{ch: make(chan struct{}, 1)}
	lk.ch <- struct{}{}
	b.locks[key] = lk
	return lk
}

func cloneTopMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type errorString string

func (e errorString) Error() string { return string(e) }
