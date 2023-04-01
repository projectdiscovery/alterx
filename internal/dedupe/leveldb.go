package dedupe

import (
	"os"
	"reflect"
	"unsafe"

	"github.com/projectdiscovery/gologger"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

type LevelDBBackend struct {
	storage *leveldb.DB
	tempdir string
}

func NewLevelDBBackend() *LevelDBBackend {
	l := &LevelDBBackend{}
	dbPath, err := os.MkdirTemp("", "nuclei-report-*")
	if err != nil {
		gologger.Fatal().Msgf("failed to create temp dir for alterx dedupe got: %v", err)
	}
	l.tempdir = dbPath
	l.storage, err = leveldb.OpenFile(dbPath, nil)
	if err != nil {
		if !errors.IsCorrupted(err) {
			gologger.Fatal().Msgf("goleveldb: failed to open db got %v", err)
		}
		// If the metadata is corrupted, try to recover
		l.storage, err = leveldb.RecoverFile(dbPath, nil)
		if err != nil {
			gologger.Fatal().Msgf("goleveldb: corrupted db found, recovery failed got %v")
		}
	}
	return l
}

func (l *LevelDBBackend) Upsert(elem string) {
	if err := l.storage.Put(unsafeToBytes(elem), nil, nil); err != nil {
		gologger.Error().Msgf("dedupe: leveldb: got %v while writing %v", err, elem)
	}
}

func (l *LevelDBBackend) IterCallback(callback func(elem string)) {
	iter := l.storage.NewIterator(nil, nil)
	for iter.Next() {
		callback(string(iter.Key()))
	}
}

func (l *LevelDBBackend) Cleanup() {
	if err := os.RemoveAll(l.tempdir); err != nil {
		gologger.Error().Msgf("leveldb: cleanup got %v", err)
	}
}

// unsafeToBytes converts a string to byte slice and does it with
// zero allocations.
//
// Reference - https://stackoverflow.com/questions/59209493/how-to-use-unsafe-get-a-byte-slice-from-a-string-without-memory-copy
func unsafeToBytes(data string) []byte {
	var buf = *(*[]byte)(unsafe.Pointer(&data))
	(*reflect.SliceHeader)(unsafe.Pointer(&buf)).Cap = len(data)
	return buf
}
