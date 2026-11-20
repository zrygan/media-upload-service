package config

import "testing"

func TestAddrs(t *testing.T) {
	d := t.TempDir()
	for _, tc := range []struct {
		a  string
		ok bool
	}{{"localhost:50051", true}, {"192.168.1.20:50051", true}, {"consumer:1", true}, {"[::1]:50051", true}, {"max", false}, {"", false}, {":50051", false}, {"h:0", false}, {"h:65536", false}, {"h:abc", false}, {"50051", false}} {
		err := ProducerConfig{ThreadCount: 1, FolderList: []string{d}, ConsumerAddress: tc.a}.Validate()
		t.Logf("%q -> %v", tc.a, err)
		if (err == nil) != tc.ok {
			t.Fatalf("%q", tc.a)
		}
	}
}
