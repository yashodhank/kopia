package storagetesting

import (
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kopia/kopia/storage"
)

type mapStorage struct {
	data  map[string][]byte
	mutex sync.RWMutex
}

func (s *mapStorage) BlockExists(id string) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, ok := s.data[string(id)]
	return ok, nil
}

func (s *mapStorage) GetBlock(id string) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	data, ok := s.data[string(id)]
	if ok {
		return data, nil
	}

	return nil, storage.ErrBlockNotFound
}

func (s *mapStorage) PutBlock(id string, data storage.ReaderWithLength, options storage.PutOptions) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.data[string(id)]; ok {
		data.Close()
		return nil
	}

	c, err := ioutil.ReadAll(data)
	data.Close()
	if err != nil {
		return err
	}

	s.data[string(id)] = c
	return nil
}

func (s *mapStorage) DeleteBlock(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.data, string(id))
	return nil
}

func (s *mapStorage) ListBlocks(prefix string) chan (storage.BlockMetadata) {
	ch := make(chan (storage.BlockMetadata))
	fixedTime := time.Now()
	go func() {
		s.mutex.RLock()
		defer s.mutex.RUnlock()

		keys := []string{}
		for k := range s.data {
			if strings.HasPrefix(k, string(prefix)) {
				keys = append(keys, k)
			}
		}

		sort.Strings(keys)

		for _, k := range keys {
			v := s.data[k]
			ch <- storage.BlockMetadata{
				BlockID:   string(k),
				Length:    uint64(len(v)),
				TimeStamp: fixedTime,
			}
		}
		close(ch)
	}()
	return ch
}

// NewMapStorage returns an implementation of Storage backed by the contents of given map.
// Used primarily for testing.
func NewMapStorage(data map[string][]byte) storage.Storage {
	return &mapStorage{data: data}
}