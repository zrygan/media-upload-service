package dedup

import "sync"

// Index holds the signatures of the videos the consumer knows.
// CheckAndInsert compares and inserts under one lock, so two uploads of
// the same video can't both pass the check.
type Index struct {
	mu         sync.Mutex
	signatures map[string][]uint64
}

// NewIndex returns an empty Index.
func NewIndex() *Index {
	// TODO: implement
	panic("NewIndex: not implemented")
}

// CheckAndInsert reports whether signature matches a stored one. If it
// doesn't, signature is stored under id.
func (idx *Index) CheckAndInsert(id string, signature []uint64) bool {
	// TODO: implement
	panic("CheckAndInsert: not implemented")
}

// Remove deletes the signature under id, so the retry of a failed upload
// isn't rejected as a duplicate of a video that was never saved.
func (idx *Index) Remove(id string) {
	// TODO: implement
	panic("Remove: not implemented")
}
