package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(file, nil, 0o644)
	cases := []struct {
		name, body string
		ok         bool
	}{
		{"valid", `{"thread_count":1,"folder_list":["` + dir + `"],"consumer_address":"consumer:50051"}`, true},
		{"malformed", `{not json`, false},
		{"zero p", `{"thread_count":0,"folder_list":[],"consumer_address":"x"}`, false},
		{"mismatch", `{"thread_count":5,"folder_list":["` + dir + `"],"consumer_address":"x"}`, false},
		{"missing folder", `{"thread_count":1,"folder_list":["/no/such/dir"],"consumer_address":"x"}`, false},
		{"file not folder", `{"thread_count":1,"folder_list":["` + file + `"],"consumer_address":"x"}`, false},
		{"no address", `{"thread_count":1,"folder_list":["` + dir + `"]}`, false},
	}
	for _, c := range cases {
		p := filepath.Join(t.TempDir(), "c.json")
		os.WriteFile(p, []byte(c.body), 0o644)
		cfg, err := ReadConfigFromJSON(p)
		t.Logf("%-16s err=%v", c.name, err)
		if c.ok != (err == nil) || c.ok != (cfg != nil) {
			t.Errorf("%s: ok=%v cfg=%v err=%v", c.name, c.ok, cfg, err)
		}
	}
	if _, err := ReadConfigFromJSON("/no/such/file.json"); err == nil {
		t.Error("missing file: expected error")
	} else {
		t.Logf("missing file     err=%v", err)
	}
}
