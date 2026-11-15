// Package inflight tracks which videos the consumer is handling right
// now, keyed by the video's SHA-256 hash in hex.
package inflight

import "sync"

// State is how far along an in-flight video is.
type State int

const (
	// Uploading means a stream is writing the video's .part file.
	Uploading State = iota + 1
	// Queued means the file is complete and waiting for a worker.
	Queued
)

// Hashbrowns is the set of in-flight videos. The upload handlers and
// the workers share one, so every method takes the lock.
type Hashbrowns struct {
	mut   sync.Mutex
	state map[string]State
}

// New returns an empty Hashbrowns.
func New() *Hashbrowns {
	return &Hashbrowns{state: make(map[string]State)}
}

// TryUpload claims hash for the caller and returns true if it got it.
// If another upload already has it, this returns false and that
// upload's state.
//
// The check and the insert happen under one lock, so two uploads of
// the same video can never both get true.
func (h *Hashbrowns) TryUpload(hash string) (State, bool) {
	h.mut.Lock()
	defer h.mut.Unlock()

	if st, taken := h.state[hash]; taken {
		return st, false
	}

	h.state[hash] = Uploading
	return Uploading, true
}

// HashQueued marks hash as Queued.
//
// Call it before queue.TryEnqueue, not after. Once the job is in the
// queue, a worker can finish it and Drop the hash, and a late
// HashQueued would then leave the hash Queued forever.
func (h *Hashbrowns) HashQueued(hash string) {
	h.mut.Lock()
	defer h.mut.Unlock()
	h.state[hash] = Queued
}

// Drop removes hash, once its upload failed or a worker finished it.
func (h *Hashbrowns) Drop(hash string) {
	h.mut.Lock()
	defer h.mut.Unlock()
	delete(h.state, hash)
}
