package usage

import "sync"

// Broadcaster sends usage entries to SSE subscribers.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan Entry]struct{}
}

var Global = &Broadcaster{
	subscribers: make(map[chan Entry]struct{}),
}

// Subscribe returns a channel that receives new usage entries.
// Call Unsubscribe when done.
func (b *Broadcaster) Subscribe() chan Entry {
	ch := make(chan Entry, 10)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes the subscriber channel.
func (b *Broadcaster) Unsubscribe(ch chan Entry) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends an entry to all active subscribers (non-blocking).
func (b *Broadcaster) Publish(e Entry) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- e:
		default: // drop if subscriber is slow
		}
	}
}
