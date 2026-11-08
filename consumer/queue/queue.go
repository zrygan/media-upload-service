package queue

// Job holds the information of one finished upload. The upload handler
// makes a Job once the whole file is in the staging folder, and a
// worker moves that file into the media folder.
type Job struct {
	// The name of the video in the producer's folder. This comes from
	// the producer, so only use its base name (filepath.Base) in paths.
	FileName string
	// Where the finished upload is in the staging folder.
	StagingPath string
	// The size of the video in bytes, already checked against the bytes
	// that were received.
	FileSize int64
	// The ID of the producer thread that uploaded the video.
	ProducerID int32
	// The SHA-256 hash of the video, as sent by the producer.
	Hash []byte
	// The perceptual hash (video dHash) of the video, as sent by the
	// producer.
	Signature []uint64
}

// Queue is the bounded leaky bucket between the upload handlers and
// the workers. It holds at most Size uploads, and a new upload is
// dropped (not blocked) when it is full.
//
// It uses two buffered channels with the same capacity:
//
//  1. reserve holds one permit for each upload that has a slot. An
//     upload takes a permit (TryReserve) before it receives any data,
//     and gives it back (UnReserve) when a worker takes its job or
//     when the upload fails.
//  2. ch holds the finished jobs for the workers.
//
// A job is only sent to ch while holding a permit, and the permit is
// only given back after the job leaves ch. So ch never has more jobs
// than permits, and TryEnqueue never fails after a TryReserve.
//
// All methods are safe to call from many goroutines, except Close.
//
// NOTE: The permits let us drop an upload at its first message. If we
// only checked ch, we would have to receive the whole video before we
// know that it does not fit.
type Queue struct {
	ch      chan Job
	reserve chan struct{}
}

// New returns an empty Queue that holds at most size uploads.
//
// NOTE: size must be positive (see ConsumerConfig.Validate). If size
// is 0, both channels are unbuffered and every upload is dropped.
func New(size int) *Queue {
	return &Queue{ch: make(chan Job, size), reserve: make(chan struct{}, size)}
}

// TryEnqueue adds j to the queue without blocking. This returns true
// if j was added, and false if the queue was full.
//
// The caller must hold a permit from TryReserve, so false means a bug
// and not a full queue (see Queue). This panics if the queue is
// closed.
func (q *Queue) TryEnqueue(j Job) bool {
	select {
	case q.ch <- j:
		return true
	default:
		return false
	}
}

// UnReserve gives back a permit taken by TryReserve. Call this exactly
// once for each successful TryReserve.
//
// NOTE: Calling this without holding a permit breaks the count. It
// blocks until another upload takes a permit, then frees THAT upload's
// permit.
func (q *Queue) UnReserve() {
	<-q.reserve
}

// CountReserve returns the number of permits taken, which is the
// number of uploads still arriving plus the number of jobs waiting
// for a worker.
//
// NOTE: The count can change right after this returns, so only use it
// for logs and status, not to decide if an upload fits. See
// TryReserve.
func (q *Queue) CountReserve() int {
	return len(q.reserve)
}

// TryReserve takes a permit without blocking. This returns true if it
// got one, and false if the queue is full, which means the upload
// should be dropped.
//
// Checking for a free slot and taking it is one channel send, so two
// uploads can never both take the last slot.
func (q *Queue) TryReserve() bool {
	select {
	case q.reserve <- struct{}{}:
		return true
	default:
		return false
	}
}

// Jobs returns the channel that the workers receive jobs from. It is
// receive-only, so the workers cannot send to it or close it.
//
// After Close, the workers still receive the jobs left in the channel,
// then receive ok == false once it is empty.
func (q *Queue) Jobs() <-chan Job {
	return q.ch
}

// Close tells the workers that no more jobs are coming.
//
// NOTE: Call this only once, and only after no upload can call
// TryEnqueue anymore (after the gRPC server stops). Sending to a
// closed channel or closing it twice panics.
func (q *Queue) Close() {
	close(q.ch)
}

// Pending returns the number of jobs waiting for a worker. Unlike
// CountReserve, this does not count uploads that are still arriving.
func (q *Queue) Pending() int {
	return len(q.ch)
}

// Size returns the maximum number of uploads the queue holds (q).
func (q *Queue) Size() int {
	return cap(q.ch)
}
