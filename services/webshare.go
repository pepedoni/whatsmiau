package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const webshareBaseURL = "https://proxy.webshare.io/api/v2"

type WebshareProxy struct {
	ID           string `json:"id"`
	ProxyAddress string `json:"proxy_address"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Valid        bool   `json:"valid"`
}

type webshareListResponse struct {
	Results []WebshareProxy `json:"results"`
	Next    *string         `json:"next"`
}

type WebshareService struct {
	apiKey   string
	client   *http.Client
	mu       sync.Mutex
	cache    []WebshareProxy
	cachedAt time.Time
	cacheTTL time.Duration
}

func NewWebshareService(apiKey string) *WebshareService {
	return &WebshareService{
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 15 * time.Second},
		cacheTTL: 5 * time.Minute,
	}
}

func (s *WebshareService) Enabled() bool {
	return s.apiKey != ""
}

func (s *WebshareService) FetchProxies() ([]WebshareProxy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.cachedAt) < s.cacheTTL && len(s.cache) > 0 {
		return s.cache, nil
	}

	proxies, err := s.fetchAllPages()
	if err != nil {
		return nil, err
	}

	s.cache = proxies
	s.cachedAt = time.Now()
	return proxies, nil
}

func (s *WebshareService) fetchAllPages() ([]WebshareProxy, error) {
	var all []WebshareProxy
	url := fmt.Sprintf("%s/proxy/list/?valid=true&mode=direct&page_size=100", webshareBaseURL)

	for url != "" {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("webshare: create request: %w", err)
		}
		req.Header.Set("Authorization", "Token "+s.apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("webshare: do request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("webshare: read body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("webshare: unexpected status %d: %s", resp.StatusCode, string(body))
		}

		var page webshareListResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("webshare: unmarshal: %w", err)
		}

		all = append(all, page.Results...)

		if page.Next != nil {
			url = *page.Next
		} else {
			url = ""
		}
	}

	return all, nil
}

// GetAvailableProxy returns a valid proxy whose ID is not in usedIDs.
// If all proxies are already used, it returns any valid proxy.
func (s *WebshareService) GetAvailableProxy(usedIDs []string) (*WebshareProxy, error) {
	proxies, err := s.FetchProxies()
	if err != nil {
		return nil, err
	}

	usedSet := make(map[string]bool, len(usedIDs))
	for _, id := range usedIDs {
		usedSet[id] = true
	}

	var fallback *WebshareProxy
	for i := range proxies {
		p := &proxies[i]
		if !p.Valid {
			continue
		}
		if fallback == nil {
			fallback = p
		}
		if !usedSet[p.ID] {
			return p, nil
		}
	}

	return fallback, nil
}
