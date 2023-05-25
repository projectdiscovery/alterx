package dedupe

import (
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/hmap/store/hybrid"
)

type LevelDBBackend struct {
	storage *hybrid.HybridMap
}

func NewLevelDBBackend() *LevelDBBackend {
	l := &LevelDBBackend{}
	db, err := hybrid.New(hybrid.DefaultDiskOptions)
	if err != nil {
		gologger.Fatal().Msgf("failed to create temp dir for alterx dedupe got: %v", err)
	}
	l.storage = db
	return l
}

func (l *LevelDBBackend) Upsert(elem string) {
	if err := l.storage.Set(elem, nil); err != nil {
		gologger.Error().Msgf("dedupe: leveldb: got %v while writing %v", err, elem)
	}
}

func (l *LevelDBBackend) IterCallback(callback func(elem string)) {
	l.storage.Scan(func(k, _ []byte) error {
		callback(string(k))
		return nil
	})
}

func (l *LevelDBBackend) Cleanup() {
	_ = l.storage.Close()
}
