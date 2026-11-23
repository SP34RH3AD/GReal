package core

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeO365 emulates GetCredentialType: addresses containing "nobody" return
// IfExistsResult 1 (not found); everything else returns 0 (exists).
func fakeO365(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var in map[string]string
		_ = json.Unmarshal(body, &in)
		res := 0
		if strings.Contains(strings.ToLower(in["Username"]), "nobody") {
			res = 1
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(credTypeResp{IfExistsResult: res})
	}))
}

func TestO365WebEnumeration(t *testing.T) {
	srv := fakeO365(t)
	defer srv.Close()
	o365CredTypeURL = srv.URL

	o := options{
		Provider:     ProviderO365,
		Technique:    TechWeb,
		TargetDomain: "example.com",
		Format:       "5",
		Threads:      4,
	}

	c := &collector{}
	accounts := []string{"alice", "bob", "nobody", "carol"}
	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < o.Threads; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); o.webWorker(jobs, c) }()
	}
	for _, a := range accounts {
		jobs <- a
	}
	close(jobs)
	wg.Wait()

	found := c.list()
	if len(found) != 3 {
		t.Fatalf("found = %d, want 3 (alice, bob, carol)", len(found))
	}
	for _, f := range found {
		if strings.Contains(f, "nobody") {
			t.Errorf("nobody should not be reported, got %q", f)
		}
	}
}
