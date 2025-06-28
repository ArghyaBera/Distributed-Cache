package cache

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Cache struct {
	lock    sync.RWMutex
	data    map[string][]byte
	expiry  map[string]time.Time
	metrics *CacheMetrics
}

type CacheMetrics struct {
	Hits    uint64
	Misses  uint64
	Sets    uint64
	Deletes uint64
}

func NewCache() *Cache {
	return &Cache{
		data:    make(map[string][]byte),
		expiry:  make(map[string]time.Time),
		metrics: &CacheMetrics{},
	}
}

func (c *Cache) Set(key, value []byte, ttl time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	strKey := string(key)
	c.data[strKey] = value
	c.metrics.Sets++

	if ttl > 0 {
		c.expiry[strKey] = time.Now().Add(ttl)
		go c.startEviction(strKey, ttl)
	}

	log.Printf("SET %s to %s (TTL: %v)\n", strKey, string(value), ttl)
	return nil
}

func (c *Cache) Get(key []byte) ([]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	strKey := string(key)
	val, ok := c.data[strKey]
	if !ok {
		c.metrics.Misses++
		return nil, fmt.Errorf("key (%s) not found", strKey)
	}

	if exp, exists := c.expiry[strKey]; exists && time.Now().After(exp) {
		c.metrics.Misses++
		go c.Delete(key) // Async delete expired key
		return nil, fmt.Errorf("key (%s) has expired", strKey)
	}

	c.metrics.Hits++
	log.Printf("GET %s = %s\n", strKey, string(val))
	return val, nil
}

func (c *Cache) Has(key []byte) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	strKey := string(key)
	if _, ok := c.data[strKey]; !ok {
		return false
	}

	if exp, exists := c.expiry[strKey]; exists && time.Now().After(exp) {
		return false
	}
	return true
}

func (c *Cache) Delete(key []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	strKey := string(key)
	delete(c.data, strKey)
	delete(c.expiry, strKey)
	c.metrics.Deletes++

	log.Printf("DELETE %s\n", strKey)
	return nil
}

func (c *Cache) Keys() [][]byte {
	c.lock.RLock()
	defer c.lock.RUnlock()

	keys := make([][]byte, 0, len(c.data))
	for k := range c.data {
		if exp, exists := c.expiry[k]; !exists || !time.Now().After(exp) {
			keys = append(keys, []byte(k))
		}
	}
	return keys
}

func (c *Cache) Metrics() *CacheMetrics {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return &CacheMetrics{
		Hits:    c.metrics.Hits,
		Misses:  c.metrics.Misses,
		Sets:    c.metrics.Sets,
		Deletes: c.metrics.Deletes,
	}
}

func (c *Cache) startEviction(key string, ttl time.Duration) {
	<-time.After(ttl)
	c.lock.Lock()
	defer c.lock.Unlock()

	if exp, exists := c.expiry[key]; exists && time.Now().After(exp) {
		delete(c.data, key)
		delete(c.expiry, key)
		c.metrics.Deletes++
		log.Printf("EVICTED %s\n", key)
	}
}

// BatchSet sets multiple key-value pairs
func (c *Cache) BatchSet(pairs map[string][]byte, ttl time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	for k, v := range pairs {
		c.data[k] = v
		c.metrics.Sets++
		if ttl > 0 {
			c.expiry[k] = time.Now().Add(ttl)
			go c.startEviction(k, ttl)
		}
		log.Printf("BATCH SET %s to %s\n", k, string(v))
	}
	return nil
}
