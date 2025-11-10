package alterx

import (
	"bytes"
	"io"
	"strings"
	"sync"

	"github.com/projectdiscovery/utils/dedupe"
)

// DedupingWriter wraps an io.Writer with transparent deduplication using dedupe utils
type DedupingWriter struct {
	writer    io.Writer
	inputCh   chan string
	blacklist map[string]bool
	wg        sync.WaitGroup
	count     int
	countMu   sync.Mutex
	closed    bool
	buffer    []byte
}

// NewDedupingWriter creates a new DedupingWriter with optional blacklist/seed
// The seed parameter allows pre-populating items to skip
func NewDedupingWriter(w io.Writer, seed ...string) *DedupingWriter {
	blacklist := make(map[string]bool, len(seed))
	for _, item := range seed {
		blacklist[item] = true
	}

	inputCh := make(chan string, 100)
	dw := &DedupingWriter{
		writer:    w,
		inputCh:   inputCh,
		blacklist: blacklist,
		buffer:    make([]byte, 0),
	}

	// Start async dedupe processing
	dw.wg.Add(1)
	go dw.processDeduped(inputCh)

	return dw
}

// processDeduped handles the dedupe output and writes to underlying writer
func (dw *DedupingWriter) processDeduped(inputCh chan string) {
	defer dw.wg.Done()

	// Create dedupe instance (it handles backend selection internally)
	d := dedupe.NewDedupe(inputCh, 1024*1024) // 1MB estimate for byte length
	d.Drain()
	outputCh := d.GetResults()

	// Read deduplicated results and write to underlying writer
	for value := range outputCh {
		// Skip if in blacklist
		if dw.blacklist[value] {
			continue
		}

		// Skip empty lines and lines starting with '-'
		if value == "" || strings.HasPrefix(value, "-") {
			continue
		}

		// Write to underlying writer
		if _, err := dw.writer.Write([]byte(value + "\n")); err != nil {
			// In a real-world scenario, we might want to handle this error
			// For now, we continue processing
			continue
		}

		// Increment count
		dw.countMu.Lock()
		dw.count++
		dw.countMu.Unlock()
	}
}

// Write implements io.Writer interface
func (dw *DedupingWriter) Write(p []byte) (int, error) {
	if dw.closed {
		return 0, io.ErrClosedPipe
	}

	originalLen := len(p)

	// Append to buffer to handle incomplete lines
	dw.buffer = append(dw.buffer, p...)

	// Process complete lines
	for {
		idx := bytes.IndexByte(dw.buffer, '\n')
		if idx == -1 {
			break
		}

		line := string(dw.buffer[:idx])
		dw.inputCh <- line
		// Drop processed line plus newline
		dw.buffer = dw.buffer[idx+1:]
	}

	// Always return original length to satisfy io.Writer contract
	return originalLen, nil
}

// Close flushes any remaining data and closes the writer
func (dw *DedupingWriter) Close() error {
	if dw.closed {
		return nil
	}
	dw.closed = true

	// Process any remaining buffered data
	if len(dw.buffer) > 0 {
		line := string(dw.buffer)
		dw.inputCh <- line
	}

	// Close input channel to signal dedupe to finish
	close(dw.inputCh)

	// Wait for dedupe processing to complete
	dw.wg.Wait()

	return nil
}

// Count returns the number of unique items written
func (dw *DedupingWriter) Count() int {
	dw.countMu.Lock()
	defer dw.countMu.Unlock()
	return dw.count
}
