package core

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// collector gathers found addresses in a thread-safe way across workers.
type collector struct {
	mu    sync.Mutex
	found []string
}

func (c *collector) add(addr string) {
	c.mu.Lock()
	c.found = append(c.found, addr)
	c.mu.Unlock()
}

func (c *collector) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.found))
	copy(out, c.found)
	return out
}

// report is the structure serialized to JSON output.
type report struct {
	Classification string    `json:"classification"`
	Tool           string    `json:"tool"`
	Domain         string    `json:"domain"`
	MXServer       string    `json:"mx_server"`
	Format         string    `json:"format"`
	Generated      time.Time `json:"generated"`
	TotalProbed    int       `json:"total_probed"`
	TotalFound     int       `json:"total_found"`
	Found          []string  `json:"found"`
}

// writeOutput persists found addresses to o.OutputFile. The format is chosen by
// extension: .json, .csv, otherwise plain text. All formats carry a CONFIDENCIAL
// classification, per the GK7 ISMS handling rules.
func (o options) writeOutput(found []string, totalProbed int) error {
	if o.OutputFile == "" {
		return nil
	}

	lower := strings.ToLower(o.OutputFile)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return o.writeJSON(found, totalProbed)
	case strings.HasSuffix(lower, ".csv"):
		return o.writeCSV(found)
	default:
		return o.writeText(found, totalProbed)
	}
}

func (o options) writeJSON(found []string, totalProbed int) error {
	if found == nil {
		found = []string{}
	}
	r := report{
		Classification: "CONFIDENCIAL",
		Tool:           "GReal",
		Domain:         o.TargetDomain,
		MXServer:       o.TargetMX,
		Format:         o.Format,
		Generated:      time.Now(),
		TotalProbed:    totalProbed,
		TotalFound:     len(found),
		Found:          found,
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(o.OutputFile, append(data, '\n'), 0o600)
}

func (o options) writeCSV(found []string) error {
	f, err := os.OpenFile(o.OutputFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"classification", "domain", "email"}); err != nil {
		return err
	}
	for _, addr := range found {
		if err := w.Write([]string{"CONFIDENCIAL", o.TargetDomain, addr}); err != nil {
			return err
		}
	}
	return w.Error()
}

func (o options) writeText(found []string, totalProbed int) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# CONFIDENCIAL\n")
	fmt.Fprintf(&b, "# GReal RCPT enumeration - %s\n", o.TargetDomain)
	fmt.Fprintf(&b, "# MX: %s | format: %s | generated: %s\n", o.TargetMX, o.Format, time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "# probed: %d | found: %d\n", totalProbed, len(found))
	for _, addr := range found {
		b.WriteString(addr)
		b.WriteByte('\n')
	}
	return os.WriteFile(o.OutputFile, []byte(b.String()), 0o600)
}
