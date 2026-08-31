package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewOutputWriterStdoutOnly(t *testing.T) {
	var stdout bytes.Buffer
	w, closer, err := newOutputWriter("", &stdout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cerr := closer.Close(); cerr != nil {
			t.Errorf("close: %v", cerr)
		}
	})

	if _, err := w.Write([]byte("scanme.sh\n")); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "scanme.sh\n" {
		t.Fatalf("stdout = %q, want scanme.sh\\n", got)
	}
}

func TestNewOutputWriterWritesStdoutAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	var stdout bytes.Buffer
	w, closer, err := newOutputWriter(path, &stdout)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.Write([]byte("one.example\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("two.example\n")); err != nil {
		t.Fatal(err)
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	if got := stdout.String(); got != "one.example\ntwo.example\n" {
		t.Fatalf("stdout = %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "one.example\ntwo.example\n" {
		t.Fatalf("file = %q", got)
	}
}

func TestNewOutputWriterOpenError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir", "out.txt")
	_, _, err := newOutputWriter(missing, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error opening nested missing directory")
	}
	if !strings.Contains(err.Error(), "no-such-dir") && !os.IsNotExist(err) {
		t.Fatalf("unexpected error: %v", err)
	}
}
