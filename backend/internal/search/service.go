package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/patrickmn/go-cache"
)

const searchTimeout = 500 * time.Millisecond

var defaultRetrieveAttributes = []string{
	"id",
	"raw_lyric_file",
	"music_names",
	"artists",
	"albums",
	"ncm_music_ids",
	"qq_music_ids",
	"spotify_ids",
	"apple_music_ids",
	"isrcs",
	"ttml_author_github",
	"ttml_author_login",
	"lyric_content",
	"translated_lyric",
	"roman_lyric",
	"updated_at",
}

type Service struct {
	client *Client
	cache  *cache.Cache
}

func NewService(client *Client) *Service {
	return &Service{client: client, cache: cache.New(5*time.Minute, 10*time.Minute)}
}

func (s *Service) Initialize(ctx context.Context) error {
	if err := s.client.EnsureIndex(ctx); err != nil {
		return classifyError(err)
	}
	if _, err := applyIndexSettings(s.client.Index()); err != nil {
		return classifyError(err)
	}
	return nil
}

func (s *Service) Rebuild(ctx context.Context) error {
	return classifyError(errors.New("rebuild is handled by sync service"))
}

func (s *Service) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return SearchResponse{}, ErrInvalidRequest
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	cacheKey := cacheKey(req)
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.(SearchResponse), nil
	}
	searchCtx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	params := &meilisearch.SearchRequest{
		Query:                req.Query,
		Limit:                int64(req.Limit),
		Offset:               int64((req.Page - 1) * req.Limit),
		AttributesToRetrieve: defaultRetrieveAttributes,
		ShowRankingScore:     true,
	}
	if len(req.Fields) > 0 {
		params.AttributesToSearchOn = req.Fields
	}
	if req.Filters != "" {
		params.Filter = req.Filters
	}
	if len(req.Sort) > 0 {
		params.Sort = req.Sort
	}
	if req.ExactMatch {
		params.MatchingStrategy = meilisearch.Last
	}
	res, err := s.client.Index().SearchWithContext(searchCtx, s.client.IndexName, params)
	if err != nil {
		if errors.Is(searchCtx.Err(), context.DeadlineExceeded) {
			return SearchResponse{}, ErrTimeout
		}
		return SearchResponse{}, ErrExternalService
	}
	hits := make([]LyricDocument, 0, len(res.Hits))
	for _, h := range res.Hits {
		hits = append(hits, decodeHit(h))
	}
	total := int64(res.EstimatedTotalHits)
	resp := SearchResponse{Hits: hits, Total: total, Page: req.Page, Limit: req.Limit, TotalPages: int((total + int64(req.Limit) - 1) / int64(req.Limit)), Query: req.Query}
	s.cache.Set(cacheKey, resp, cache.DefaultExpiration)
	return resp, nil
}

func cacheKey(req SearchRequest) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%s|%v|%s|%v|%d|%d|%t", req.Query, req.Fields, req.Filters, req.Sort, req.Page, req.Limit, req.ExactMatch)))
	return fmt.Sprintf("search:%x", h.Sum64())
}

func decodeHit(h meilisearch.Hit) LyricDocument {
	var doc LyricDocument
	b, err := json.Marshal(h)
	if err != nil {
		return doc
	}
	_ = json.Unmarshal(b, &doc)
	return doc
}
