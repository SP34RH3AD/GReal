package core

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeMX is a minimal SMTP server: it accepts any recipient except those
// containing "nobody", which it rejects with "does not exist.".
func fakeMX(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go serveConn(conn)
		}
	}()
	return ln.Addr().String(), func() {
		close(done)
		ln.Close()
		wg.Wait()
	}
}

func serveConn(conn net.Conn) {
	defer conn.Close()
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	write := func(s string) { w.WriteString(s + "\r\n"); w.Flush() }
	write("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			write("250 ok")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			write("250 2.1.0 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			if strings.Contains(strings.ToLower(line), "nobody") {
				write("550 5.1.1 User does not exist.")
			} else {
				write("250 2.1.5 OK")
			}
		case strings.HasPrefix(cmd, "RSET"):
			write("250 ok")
		case strings.HasPrefix(cmd, "QUIT"):
			write("221 bye")
			return
		default:
			write("250 ok")
		}
	}
}

func TestEnumerationEndToEnd(t *testing.T) {
	addr, stop := fakeMX(t)
	defer stop()
	host, port, _ := net.SplitHostPort(addr)
	smtpPort = port // point probes at the fake server's port

	out := filepath.Join(t.TempDir(), "results.json")
	o := options{
		Provider:     ProviderGeneric,
		Technique:    TechRCPT,
		TargetDomain: "example.com",
		TargetMX:     host,
		Format:       "5",
		Threads:      4,
		OutputFile:   out,
		MaxPerConn:   2, // force connection recycling mid-run
	}

	c := &collector{}
	accounts := []string{"alice", "bob", "nobody", "carol", "nobody", "dave"}
	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < o.Threads; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); o.worker(jobs, c) }()
	}
	for _, a := range accounts {
		jobs <- a
	}
	close(jobs)
	wg.Wait()

	if err := o.writeOutput(c.list(), len(accounts)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Classification != "CONFIDENCIAL" {
		t.Errorf("classification = %q, want CONFIDENCIAL", r.Classification)
	}
	if r.TotalFound != 4 {
		t.Errorf("found = %d, want 4 (alice,bob,carol,dave)", r.TotalFound)
	}
	for _, addr := range r.Found {
		if strings.Contains(addr, "nobody") {
			t.Errorf("nobody should have been rejected, got %q", addr)
		}
	}
}
