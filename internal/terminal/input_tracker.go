package terminal

import (
	"bytes"
	"io"
	"sync"
)

const recentInputLimit = 4 * 1024

type inputTracker struct {
	mu     sync.Mutex
	recent []byte
}

func (t *inputTracker) Record(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, value := range data {
		switch value {
		case '\r', '\n', 0x03, 0x1c:
			t.recent = t.recent[:0]
		default:
			t.recent = append(t.recent, value)
			if len(t.recent) > recentInputLimit {
				copy(t.recent, t.recent[len(t.recent)-recentInputLimit:])
				t.recent = t.recent[:recentInputLimit]
			}
		}
	}
}

func (t *inputTracker) MatchesEchoPrefix(echo []byte) bool {
	if len(echo) == 0 {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.recent) >= len(echo) && bytes.Equal(t.recent[:len(echo)], echo)
}

func (t *inputTracker) ConsumeEchoPrefix(echo []byte) bool {
	if len(echo) == 0 {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.recent) < len(echo) || !bytes.Equal(t.recent[:len(echo)], echo) {
		return false
	}
	copy(t.recent, t.recent[len(echo):])
	t.recent = t.recent[:len(t.recent)-len(echo)]
	return true
}

type trackingReader struct {
	reader  io.Reader
	tracker *inputTracker
}

func (r trackingReader) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	if n > 0 {
		r.tracker.Record(data[:n])
	}
	return n, err
}
