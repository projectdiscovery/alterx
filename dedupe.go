package alterx

import "github.com/projectdiscovery/alterx/internal/dedupe"

// MaxInMemoryDedupeSize (default : 100 MB)
var MaxInMemoryDedupeSize = 100 * 1024 * 1024

type DedupeBackend interface {
	// Upsert add/update key to backend/database
	Upsert(elem string)
	// Execute given callback on each element while iterating
	IterCallback(callback func(elem string))
	// Cleanup cleans any residuals after deduping
	Cleanup()
}

// Dedupe is string deduplication type which removes
// all duplicates if
type Dedupe struct {
	receive <-chan string
	backend DedupeBackend
}

// Drains channel and tries to dedupe it
func (d *Dedupe) Drain() {
	for {
		val, ok := <-d.receive
		if !ok {
			break
		}
		d.backend.Upsert(val)
	}
}

// GetResults iterates over dedupe storage and returns results
func (d *Dedupe) GetResults() <-chan string {
	send := make(chan string, 100)
	go func() {
		defer close(send)
		d.backend.IterCallback(func(elem string) {
			send <- elem
		})
		d.backend.Cleanup()
	}()
	return send
}

// NewDedupe returns a dedupe instance which removes all duplicates
// Note: If byteLen is not correct/specified alterx may consume lot of memory
func NewDedupe(ch <-chan string, byteLen int) *Dedupe {
	d := &Dedupe{
		receive: ch,
	}
	if byteLen <= MaxInMemoryDedupeSize {
		d.backend = dedupe.NewMapBackend()
	} else {
		// gologger print a info message here
		d.backend = dedupe.NewLevelDBBackend()
	}
	return d
}
