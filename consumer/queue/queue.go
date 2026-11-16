package queue

// Job is one finished upload, waiting for a worker to save it.
type Job struct {
	// The name from the producer. Untrusted, so use filepath.Base in
	// paths.
	FileName string
	// The complete file in the staging folder.
	StagingPath string
	// The size in bytes, already checked.
	FileSize int64
	// The producer thread that sent it.
	ProducerID int32
	// The SHA-256 hash of the video.
	Hash []byte
	// The perceptual hash (video dHash), for duplicate detection.
	Signature []uint64
}

// Queue is the bounded leaky bucket between the upload handlers and the
// workers. It holds at most Size uploads. When it is full, new uploads
// are dropped, not blocked.
//
// It uses two buffered channels of the same capacity:
//
//  1. reserve holds one permit per upload that has a slot. An upload
//     takes one (TryReserve) at its first message, before any data, and
//     gives it back (UnReserve) when a worker takes the job or the
//     upload fails.
//  2. ch holds the finished jobs for the workers.
//
// A job enters ch only while holding a permit, and the permit is freed
// only after the job leaves ch, so TryEnqueue never fails after a
// TryReserve.
//
// All methods are safe for concurrent use, except Close.
type Queue struct {
	ch      chan Job
	reserve chan struct{}
}

// New returns an empty Queue that holds at most size uploads. size must
// be positive. With 0, every upload would be dropped.
func New(size int) *Queue {
	return &Queue{ch: make(chan Job, size), reserve: make(chan struct{}, size)}
}

// TryEnqueue adds j without blocking and reports whether it fit. The
// caller must hold a permit, so false means a bug, not a full queue.
// It panics if the queue is closed.
func (q *Queue) TryEnqueue(j Job) bool {
	select {
	case q.ch <- j:
		return true
	default:
		return false
	}
}

// UnReserve gives back one permit. Call it exactly once per successful
// TryReserve. An extra call blocks, then frees another upload's permit.
func (q *Queue) UnReserve() {
	<-q.reserve
}

// CountReserve returns the permits taken (uploads arriving plus jobs
// waiting). It is a snapshot, so use it for logs, not decisions.
func (q *Queue) CountReserve() int {
	return len(q.reserve)
}

// TryReserve takes a permit without blocking and reports whether it got
// one. false means the queue is full and the upload should be dropped.
// The check and the take are one channel send, so two uploads can never
// both get the last slot.
func (q *Queue) TryReserve() bool {
	select {
	case q.reserve <- struct{}{}:
		return true
	default:
		return false
	}
}

// Jobs returns the receive-only channel the workers read from. After
// Close, the workers still get the remaining jobs, then ok == false.
func (q *Queue) Jobs() <-chan Job {
	return q.ch
}

// Close tells the workers no more jobs are coming. Call it once, after
// the gRPC server has stopped. Sending to or closing a closed channel
// panics.
func (q *Queue) Close() {
	close(q.ch)
}

// Pending returns the jobs waiting for a worker, not counting uploads
// still arriving.
func (q *Queue) Pending() int {
	return len(q.ch)
}

// Size returns the queue's capacity (q).
func (q *Queue) Size() int {
	return cap(q.ch)
}
