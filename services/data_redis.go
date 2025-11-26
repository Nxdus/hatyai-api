package services

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisKeyRaw = "request:data:raw"
	redisTTL    = 60 * time.Second
)

type SOSService interface {
	GetRaw() ([]byte, error)
	GetSOS() (*APIResponse, error)
}

type redisSOSService struct {
	redis    *redis.Client
	fetcher  APIFetcher
	refreshM sync.Mutex
	memCache atomic.Value
}

type cachedPayload struct {
	ETag string          `json:"etag"`
	JSON json.RawMessage `json:"json"`
}

type memoryCache struct {
	raw      []byte
	parsed   *APIResponse
	etag     string
	expires  time.Time
}

func NewRedisSOSService(redis *redis.Client, fetcher APIFetcher) SOSService {
	return &redisSOSService{
		redis:   redis,
		fetcher: fetcher,
	}
}

func (s *redisSOSService) GetRaw() ([]byte, error) {
	if cached := s.loadMemoryCache(); cached != nil && time.Now().Before(cached.expires) {
		s.tryRefresh(cached.etag)
		return cached.raw, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	val, err := s.redis.Get(ctx, redisKeyRaw).Bytes()
	if err == nil {
		var cached cachedPayload
		if json.Unmarshal(val, &cached) == nil {
			s.storeMemoryCache(cached.JSON, cached.ETag, nil)
			s.tryRefresh(cached.ETag)
			return cached.JSON, nil
		}
	}

	data, etag, _, err := s.fetcher.Fetch("")
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errors.New("no data returned from fetcher")
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	s.saveRawCache(etag, raw, data)
	return raw, nil
}

func (s *redisSOSService) GetSOS() (*APIResponse, error) {
	if cached := s.loadMemoryCache(); cached != nil && cached.parsed != nil && time.Now().Before(cached.expires) {
		return cached.parsed, nil
	}

	raw, err := s.GetRaw()
	if err != nil {
		return nil, err
	}

	var data APIResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	s.updateParsedCache(&data)
	return &data, nil
}

func (s *redisSOSService) tryRefresh(etag string) {
	if !s.refreshM.TryLock() {
		return
	}

	go func() {
		defer s.refreshM.Unlock()

		data, newETag, notModified, err := s.fetcher.Fetch(etag)
		if err != nil {
			log.Printf("cache refresh failed: %v", err)
			return
		}
		if notModified || data == nil {
			s.touchCache(etag)
			return
		}

		raw, err := json.Marshal(data)
		if err != nil {
			return
		}
		s.saveRawCache(newETag, raw, data)
	}()
}

func (s *redisSOSService) saveRawCache(etag string, raw []byte, parsed *APIResponse) {
	payload := cachedPayload{
		ETag: etag,
		JSON: raw,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal cache failed: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.redis.Set(ctx, redisKeyRaw, bytes, redisTTL).Err(); err != nil {
		log.Printf("failed to save redis key=%s: %v", redisKeyRaw, err)
	} else {
		log.Printf("redis cache updated (etag=%s, ttl=%s)", etag, redisTTL)
	}

	s.storeMemoryCache(raw, etag, parsed)
}

func (s *redisSOSService) storeMemoryCache(raw []byte, etag string, parsed *APIResponse) {
	ttl := redisTTL - 5*time.Second
	if ttl < 5*time.Second {
		ttl = redisTTL
	}

	s.memCache.Store(&memoryCache{
		raw:     raw,
		parsed:  parsed,
		etag:    etag,
		expires: time.Now().Add(ttl),
	})
}

func (s *redisSOSService) loadMemoryCache() *memoryCache {
	val := s.memCache.Load()
	if val == nil {
		return nil
	}
	if cached, ok := val.(*memoryCache); ok {
		return cached
	}
	return nil
}

func (s *redisSOSService) updateParsedCache(parsed *APIResponse) {
	current := s.loadMemoryCache()
	if current == nil {
		return
	}
	s.memCache.Store(&memoryCache{
		raw:     current.raw,
		parsed:  parsed,
		etag:    current.etag,
		expires: current.expires,
	})
}

func (s *redisSOSService) touchCache(etag string) {
	current := s.loadMemoryCache()
	if current == nil || len(current.raw) == 0 {
		return
	}
	s.saveRawCache(etag, current.raw, current.parsed)
}
