package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	// "strconv"
	"sync"
	"time"
)

// Cache is a struct that represents an in-memory cache.
type Cache struct {
	mutex      sync.Mutex
	cache      map[string]cacheValue
	capacity   int
	defaultTTL time.Duration
}

type cacheValue struct {
	value     interface{}
	expiry    time.Time
	permanent bool
}

// NewCache creates a new Cache with the specified capacity and default TTL.
func NewCache(capacity int, defaultTTL time.Duration) *Cache {
	return &Cache{
		cache:      make(map[string]cacheValue),
		capacity:   capacity,
		defaultTTL: defaultTTL,
	}
}

// Set adds a new key-value pair to the Cache.
func (c *Cache) Set(key string, value interface{}, ttl ...time.Duration) error {
	var expiry time.Time
	if len(ttl) == 0 {
		expiry = time.Now().Add(c.defaultTTL)
	} else {
		expiry = time.Now().Add(ttl[0])
	}

	if c.capacity > 0 && len(c.cache) >= c.capacity {
		c.evictOldest()
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache[key] = cacheValue{
		value:  value,
		expiry: expiry,
	}

	return nil
}

// Get retrieves the value associated with the specified key from the Cache.
func (c *Cache) Get(key string) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if value, ok := c.cache[key]; ok {
		if value.expiry.Before(time.Now()) && !value.permanent {
			delete(c.cache, key)
			return nil, errors.New("key not found")
		}
		return value.value, nil
	}

	return nil, errors.New("key not found")
}

// Delete removes the specified key-value pair from the Cache.
func (c *Cache) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, ok := c.cache[key]; ok {
		delete(c.cache, key)
		return nil
	}

	return fmt.Errorf("key %q not found", key)
}

// evictOldest removes the oldest key-value pair from the Cache.
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestExpiry time.Time

	for key, value := range c.cache {
		if oldestExpiry.IsZero() || value.expiry.Before(oldestExpiry) {
			oldestKey = key
			oldestExpiry = value.expiry
		}
	}

	delete(c.cache, oldestKey)
}

// Server is a struct that represents the HTTP server that serves the Cache.
type Server struct {
	cache *Cache
}

// SetHandler handles requests to add a new key-value pair to the Cache.
func (s *Server) SetHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")
	ttlStr := r.URL.Query().Get("ttl")

	if key == "" || value == "" {
		http.Error(w, "key and value are required", http.StatusBadRequest)
		return
	}

	var ttl time.Duration
// parse TTL from the request, if it's provided
if ttlStr != "" {
	var err error
	ttl, err = time.ParseDuration(ttlStr)
	if err != nil {
	http.Error(w, "invalid ttl", http.StatusBadRequest)
	return
	}
	}

	err := s.cache.Set(key, value, ttl)
if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
}

w.WriteHeader(http.StatusCreated)
}

// GetHandler handles requests to retrieve the value associated with a key from the Cache.
func (s *Server) GetHandler(w http.ResponseWriter, r *http.Request) {
key := r.URL.Query().Get("key")
if key == "" {
	http.Error(w, "key is required", http.StatusBadRequest)
	return
}

value, err := s.cache.Get(key)
if err != nil {
	http.Error(w, err.Error(), http.StatusNotFound)
	return
}

jsonValue, err := json.Marshal(value)
if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
}

w.Header().Set("Content-Type", "application/json")
w.Write(jsonValue)
}

// DeleteHandler handles requests to remove a key-value pair from the Cache.
func (s *Server) DeleteHandler(w http.ResponseWriter, r *http.Request) {
key := r.URL.Query().Get("key")

if key == "" {
	http.Error(w, "key is required", http.StatusBadRequest)
	return
}

err := s.cache.Delete(key)
if err != nil {
	http.Error(w, err.Error(), http.StatusNotFound)
	return
}

w.WriteHeader(http.StatusNoContent)
}

func main() {
cache := NewCache(10, 5*time.Minute)
server := &Server{cache}
http.HandleFunc("/set", server.SetHandler)
http.HandleFunc("/get", server.GetHandler)
http.HandleFunc("/delete", server.DeleteHandler)

log.Fatal(http.ListenAndServe(":8080", nil))

}
