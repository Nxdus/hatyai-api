package services

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

const apiURL = "https://storage.googleapis.com/pple-media/hdy-flood/sos.json"

type APIFetcher interface {
	Fetch(etag string) (*APIResponse, string, bool, error)
}

type httpFetcher struct {
	client *http.Client
}

func NewHTTPFetcher() APIFetcher {
	return &httpFetcher{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (h *httpFetcher) Fetch(etag string) (*APIResponse, string, bool, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, "", false, err
	}

	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		log.Printf("Upstream not modified (etag=%s)", etag)
		return nil, etag, true, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", false, errors.New("Upstream API error, Status: " + resp.Status)
	}

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", false, err
	}

	newETag := resp.Header.Get("ETag")
	log.Printf("Fetched %d items from upstream (%s, status=%s, etag=%s)", len(result.Data.Data), apiURL, resp.Status, newETag)

	return &result, newETag, false, nil
}
