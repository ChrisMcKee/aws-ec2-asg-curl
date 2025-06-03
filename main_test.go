package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseFlagsDefaults(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg, err := parseFlags(fs, []string{"-asg-name", "asg1", "-region", "eu-west-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ASGName != "asg1" || cfg.Region != "eu-west-2" {
		t.Errorf("unexpected asg-name or region: got %v", cfg)
	}
	if cfg.Path != "/" {
		t.Errorf("expected default path '/', got %q", cfg.Path)
	}
	if cfg.Port != "80" {
		t.Errorf("expected default port '80', got %q", cfg.Port)
	}
	if cfg.TLSEnabled {
		t.Errorf("expected default TLSEnabled false")
	}
	if cfg.PostFile != "" {
		t.Errorf("expected default PostFile '', got %q", cfg.PostFile)
	}
	if cfg.RequestType != "application/json" {
		t.Errorf("expected default RequestType 'application/json', got %q", cfg.RequestType)
	}
	if cfg.Timeout != 3*time.Second {
		t.Errorf("expected default Timeout 3s, got %v", cfg.Timeout)
	}
}

func TestParseFlagsMissingRequired(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_, err := parseFlags(fs, []string{})
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestParseFlagsPostFileNotExist(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_, err := parseFlags(fs, []string{"-asg-name", "asg1", "-region", "eu-west-2", "-post", "notfound.json"})
	if err == nil || err.Error() == "" || !contains(err.Error(), "POST file does not exist") {
		t.Errorf("expected error containing 'POST file does not exist', got %v", err)
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (contains(s[1:], substr) || contains(s[:len(s)-1], substr))))) || (len(s) > 0 && contains(s[1:], substr))
}

func TestParseFlagsPathFormatting(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg, err := parseFlags(fs, []string{"-asg-name", "asg1", "-region", "eu-west-2", "-path", "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Path != "/foo" {
		t.Errorf("expected path '/foo', got %q", cfg.Path)
	}
}

func TestPrintResults(t *testing.T) {
	results := []Result{
		{InstanceID: "i-1", IP: "10.0.0.1", LaunchTime: time.Unix(0, 0), ResponseTime: 100 * time.Millisecond, Error: nil, InstanceState: "stopped"},
		{InstanceID: "i-2", IP: "10.0.0.2", LaunchTime: time.Unix(0, 0), ResponseTime: 200 * time.Millisecond, Error: errors.New("fail"), InstanceState: "stopped"},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()

	printResults(results)
	w.Close()
	<-done
	os.Stdout = stdout

	output := buf.String()
	if !strings.Contains(output, "i-1") || !strings.Contains(output, "i-2") {
		t.Errorf("output missing instance IDs: %s", output)
	}
	if !strings.Contains(output, "Skipped") {
		t.Errorf("output missing 'Skipped' status for non-running instances: %s", output)
	}
}

func TestMakeRequestsEmpty(t *testing.T) {
	cfg := &Config{Timeout: 10 * time.Millisecond}
	results := makeRequests(cfg, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
