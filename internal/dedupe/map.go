package dedupe

import "runtime/debug"

type MapBackend struct {
	storage map[string]struct{}
}

func NewMapBackend() *MapBackend {
	return &MapBackend{storage: map[string]struct{}{}}
}

func (m *MapBackend) Upsert(elem string) {
	m.storage[elem] = struct{}{}
}

func (m *MapBackend) IterCallback(callback func(elem string)) {
	for k := range m.storage {
		callback(k)
	}
}

func (m *MapBackend) Cleanup() {
	m.storage = nil
	// By default GC doesnot release buffered/allocated memory
	// since there always is possibilitly of needing it again/immediately
	// and releases memory in chunks
	// debug.FreeOSMemory forces GC to release allocated memory at once
	debug.FreeOSMemory()
}
