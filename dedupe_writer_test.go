package alterx

import (
	"bytes"
	"testing"
	"time"
)

func TestDedupingWriter(t *testing.T) {
	t.Run("basic deduplication using dedupe utils", func(t *testing.T) {
		buf := &bytes.Buffer{}
		dw := NewDedupingWriter(buf)

		// Write some duplicate data
		dw.Write([]byte("test1\n"))
		dw.Write([]byte("test2\n"))
		dw.Write([]byte("test1\n")) // duplicate
		dw.Write([]byte("test3\n"))
		dw.Write([]byte("test2\n")) // duplicate

		// Close to flush and wait for async processing
		dw.Close()

		// Give a moment for async processing to complete
		time.Sleep(100 * time.Millisecond)

		if dw.Count() != 3 {
			t.Errorf("Expected 3 unique items, got %d", dw.Count())
		}

		output := buf.String()
		// Check all unique items are present (order may vary due to async)
		if !contains(output, "test1\n") || !contains(output, "test2\n") || !contains(output, "test3\n") {
			t.Errorf("Expected all unique items in output, got %q", output)
		}
	})

	t.Run("with blacklist/seed", func(t *testing.T) {
		buf := &bytes.Buffer{}
		dw := NewDedupingWriter(buf, "test1", "test3")

		// Write data including items in blacklist
		dw.Write([]byte("test1\n")) // in blacklist
		dw.Write([]byte("test2\n"))
		dw.Write([]byte("test3\n")) // in blacklist
		dw.Write([]byte("test4\n"))

		dw.Close()
		time.Sleep(100 * time.Millisecond)

		if dw.Count() != 2 {
			t.Errorf("Expected 2 unique items (excluding blacklist), got %d", dw.Count())
		}

		output := buf.String()
		// Should not contain blacklisted items
		if contains(output, "test1\n") || contains(output, "test3\n") {
			t.Errorf("Output should not contain blacklisted items, got %q", output)
		}
		// Should contain non-blacklisted items
		if !contains(output, "test2\n") || !contains(output, "test4\n") {
			t.Errorf("Output should contain test2 and test4, got %q", output)
		}
	})

	t.Run("skip lines starting with dash", func(t *testing.T) {
		buf := &bytes.Buffer{}
		dw := NewDedupingWriter(buf)

		dw.Write([]byte("test1\n"))
		dw.Write([]byte("-skip-this\n"))
		dw.Write([]byte("test2\n"))
		dw.Write([]byte("-skip-that\n"))

		dw.Close()
		time.Sleep(100 * time.Millisecond)

		if dw.Count() != 2 {
			t.Errorf("Expected 2 unique items (excluding dash lines), got %d", dw.Count())
		}

		output := buf.String()
		if contains(output, "-skip") {
			t.Errorf("Output should not contain lines starting with dash, got %q", output)
		}
	})

	t.Run("handle multiple lines in single write", func(t *testing.T) {
		buf := &bytes.Buffer{}
		dw := NewDedupingWriter(buf)

		// Write multiple lines at once with duplicates
		dw.Write([]byte("test1\ntest2\ntest1\ntest3\n"))

		dw.Close()
		time.Sleep(100 * time.Millisecond)

		if dw.Count() != 3 {
			t.Errorf("Expected 3 unique items, got %d", dw.Count())
		}

		output := buf.String()
		if !contains(output, "test1\n") || !contains(output, "test2\n") || !contains(output, "test3\n") {
			t.Errorf("Expected all unique items in output, got %q", output)
		}
	})

	t.Run("skip empty lines", func(t *testing.T) {
		buf := &bytes.Buffer{}
		dw := NewDedupingWriter(buf)

		dw.Write([]byte("test1\n\ntest2\n\n"))

		dw.Close()
		time.Sleep(100 * time.Millisecond)

		if dw.Count() != 2 {
			t.Errorf("Expected 2 unique items (skipping empty), got %d", dw.Count())
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
