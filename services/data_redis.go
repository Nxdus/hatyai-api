package services

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
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
}

type cachedPayload struct {
	ETag string          `json:"etag"`
	JSON json.RawMessage `json:"json"`
}

func NewRedisSOSService(redis *redis.Client, fetcher APIFetcher) SOSService {
	return &redisSOSService{
		redis:   redis,
		fetcher: fetcher,
	}
}

func (s *redisSOSService) GetRaw() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	val, err := s.redis.Get(ctx, redisKeyRaw).Bytes()
	if err == nil {
		var cached cachedPayload
		if json.Unmarshal(val, &cached) == nil {
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

	s.saveRawCache(etag, raw)
	return raw, nil
}

func (s *redisSOSService) GetSOS() (*APIResponse, error) {
	raw, err := s.GetRaw()
	if err != nil {
		return nil, err
	}

	var data APIResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
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
			return
		}

		raw, err := json.Marshal(data)
		if err != nil {
			return
		}
		s.saveRawCache(newETag, raw)
	}()
}

func (s *redisSOSService) saveRawCache(etag string, raw []byte) {
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
}
