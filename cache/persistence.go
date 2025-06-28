package cache

import (
	"encoding/gob"
	"os"
	"sync"
)

type PersistentCache struct {
	*Cache
	filePath string
	lock     sync.Mutex
}

func NewPersistentCache(filePath string) (*PersistentCache, error) {
	c := &PersistentCache{
		Cache:    NewCache(),
		filePath: filePath,
	}

	// Load existing cache from disk if exists
	if _, err := os.Stat(filePath); err == nil {
		if err := c.loadFromDisk(); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *PersistentCache) loadFromDisk() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	file, err := os.Open(c.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	return decoder.Decode(&c.data)
}

func (c *PersistentCache) SaveToDisk() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	file, err := os.Create(c.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(c.data)
}
