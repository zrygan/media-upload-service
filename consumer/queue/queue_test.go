package queue

import "testing"

// A queue of size 2 should accept two jobs and drop the third
// (leaky bucket: full means drop, not block).
func TestTryEnqueueDropsWhenFull(t *testing.T) {
	q := New(2)

	want := []bool{true, true, false}
	for i, w := range want {
		got := q.TryEnqueue(Job{FileName: "video.mp4"})
		if got != w {
			t.Errorf("TryEnqueue call %d: got %v, want %v", i+1, got, w)
		}
	}

	if q.Pending() != 2 {
		t.Errorf("Pending() = %d, want 2", q.Pending())
	}
}
