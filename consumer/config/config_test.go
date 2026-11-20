package config

import "testing"

func TestConsumer(t *testing.T) {
	d := t.TempDir()
	ok := ConsumerConfig{QueueMaxSize: 1, ThreadCount: 1, ListenAddress: ":50051", MediaFolder: d, StagingFolder: d}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []func(*ConsumerConfig){
		func(c *ConsumerConfig) { c.QueueMaxSize = 0 },
		func(c *ConsumerConfig) { c.ThreadCount = 0 },
		func(c *ConsumerConfig) { c.ListenAddress = "" },
		func(c *ConsumerConfig) { c.ListenAddress = "50051" },
	} {
		c := ok
		f(&c)
		err := c.Validate()
		t.Log(err)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}
