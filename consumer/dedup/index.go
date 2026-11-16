package dedup

import (
	"slices"
	"sync"
)

// Index stores the signatures of the videos known to the consumer.
//
// NOTE: The mutex is held for the comparison and the insert in
// CheckAndInsert as one critical section, this prevents two simultaneous
// uploads of the same video from both passing the duplicate check.
type Index struct {
	mu         sync.Mutex
	signatures map[string][]uint64
}

// NewIndex returns an empty Index.
func NewIndex() *Index {
	return &Index{
		signatures: make(map[string][]uint64),
	}
}

// CheckAndInsert compares signature against the signatures in the index.
// This returns true if a duplicate is found; otherwise, it stores
// signature under id and returns false.
func (idx *Index) CheckAndInsert(id string, signature []uint64) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.signatures[id]; exists {
		return true
	}

	for _, existing := range idx.signatures {
		if isDuplicate(signature, existing) {
			return true
		}
	}

	idx.signatures[id] = slices.Clone(signature)
	return false
}

// Remove deletes the signature stored under id. This is for uploads that
// failed after CheckAndInsert, so that retrying the upload is not
// rejected as a duplicate of a video that was never saved.
func (idx *Index) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	delete(idx.signatures, id)
}
