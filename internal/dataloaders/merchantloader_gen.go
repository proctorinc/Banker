// Code generated by github.com/vektah/dataloaden, DO NOT EDIT.

package dataloaders

import (
	"sync"
	"time"

	"github.com/proctorinc/banker/internal/db"
)

// MerchantLoaderConfig captures the config to create a new MerchantLoader
type MerchantLoaderConfig struct {
	// Fetch is a method that provides the data for the loader
	Fetch func(keys []string) ([]db.Merchant, []error)

	// Wait is how long wait before sending a batch
	Wait time.Duration

	// MaxBatch will limit the maximum number of keys to send in one batch, 0 = not limit
	MaxBatch int
}

// NewMerchantLoader creates a new MerchantLoader given a fetch, wait, and maxBatch
func NewMerchantLoader(config MerchantLoaderConfig) *MerchantLoader {
	return &MerchantLoader{
		fetch:    config.Fetch,
		wait:     config.Wait,
		maxBatch: config.MaxBatch,
	}
}

// MerchantLoader batches and caches requests
type MerchantLoader struct {
	// this method provides the data for the loader
	fetch func(keys []string) ([]db.Merchant, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// INTERNAL

	// lazily created cache
	cache map[string]db.Merchant

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *merchantLoaderBatch

	// mutex to prevent races
	mu sync.Mutex
}

type merchantLoaderBatch struct {
	keys    []string
	data    []db.Merchant
	error   []error
	closing bool
	done    chan struct{}
}

// Load a Merchant by key, batching and caching will be applied automatically
func (l *MerchantLoader) Load(key string) (db.Merchant, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a Merchant.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *MerchantLoader) LoadThunk(key string) func() (db.Merchant, error) {
	l.mu.Lock()
	if it, ok := l.cache[key]; ok {
		l.mu.Unlock()
		return func() (db.Merchant, error) {
			return it, nil
		}
	}
	if l.batch == nil {
		l.batch = &merchantLoaderBatch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)
	l.mu.Unlock()

	return func() (db.Merchant, error) {
		<-batch.done

		var data db.Merchant
		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		// its convenient to be able to return a single error for everything
		if len(batch.error) == 1 {
			err = batch.error[0]
		} else if batch.error != nil {
			err = batch.error[pos]
		}

		if err == nil {
			l.mu.Lock()
			l.unsafeSet(key, data)
			l.mu.Unlock()
		}

		return data, err
	}
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *MerchantLoader) LoadAll(keys []string) ([]db.Merchant, []error) {
	results := make([]func() (db.Merchant, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	merchants := make([]db.Merchant, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		merchants[i], errors[i] = thunk()
	}
	return merchants, errors
}

// LoadAllThunk returns a function that when called will block waiting for a Merchants.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *MerchantLoader) LoadAllThunk(keys []string) func() ([]db.Merchant, []error) {
	results := make([]func() (db.Merchant, error), len(keys))
	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]db.Merchant, []error) {
		merchants := make([]db.Merchant, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			merchants[i], errors[i] = thunk()
		}
		return merchants, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *MerchantLoader) Prime(key string, value db.Merchant) bool {
	l.mu.Lock()
	var found bool
	if _, found = l.cache[key]; !found {
		l.unsafeSet(key, value)
	}
	l.mu.Unlock()
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *MerchantLoader) Clear(key string) {
	l.mu.Lock()
	delete(l.cache, key)
	l.mu.Unlock()
}

func (l *MerchantLoader) unsafeSet(key string, value db.Merchant) {
	if l.cache == nil {
		l.cache = map[string]db.Merchant{}
	}
	l.cache[key] = value
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *merchantLoaderBatch) keyIndex(l *MerchantLoader, key string) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go b.startTimer(l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go b.end(l)
		}
	}

	return pos
}

func (b *merchantLoaderBatch) startTimer(l *MerchantLoader) {
	time.Sleep(l.wait)
	l.mu.Lock()

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		l.mu.Unlock()
		return
	}

	l.batch = nil
	l.mu.Unlock()

	b.end(l)
}

func (b *merchantLoaderBatch) end(l *MerchantLoader) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}
