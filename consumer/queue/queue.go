package queue

type Job struct {
	FileName    string
	StagingPath string
	FileSize    int64
	ProducerID  int32
	Hash        []byte
	Signature   []uint64
}

type Queue struct {
	ch chan Job
}

func New(size int) *Queue {
	return &Queue{ch: make(chan Job, size)}
}

func (q *Queue) TryEnqueue(j Job) bool {
	select {
	case q.ch <- j:
		return true
	default:
		return false
	}
}

func (q *Queue) Jobs() <-chan Job {
	return q.ch
}

func (q *Queue) Close() {
	close(q.ch)
}

func (q *Queue) Pending() int {
	return len(q.ch)
}

func (q *Queue) Size() int {
	return cap(q.ch)
}
