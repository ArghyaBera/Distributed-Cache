package cache

import "time"

type Cacher interface {
	Set([]byte, []byte, time.Duration) error
	BatchSet(map[string][]byte, time.Duration) error
	Has([]byte) bool
	Get([]byte) ([]byte, error)
	Delete([]byte) error
	Keys() [][]byte
	Metrics() *CacheMetrics
}
