package handler

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"amllhub/backend/internal/search"
	"github.com/gin-gonic/gin"
)

type SearchService interface {
	Search(ctx context.Context, req search.SearchRequest) (search.SearchResponse, error)
}

type RebuildService interface {
	Rebuild(ctx context.Context) error
}

type SearchHandler struct {
	service   SearchService
	rebuild   RebuildService
	lastRun   time.Time
	mu        sync.Mutex
	rateLimit time.Duration
}

func NewSearch(service SearchService, rebuild RebuildService, rateLimit time.Duration) *SearchHandler {
	return &SearchHandler{service: service, rebuild: rebuild, rateLimit: rateLimit}
}

func (h *SearchHandler) Search(c *gin.Context) {
	var req search.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": 400})
		return
	}
	start := time.Now()
	resp, err := h.service.Search(c.Request.Context(), req)
	if err != nil {
		c.JSON(statusFromSearchError(err), gin.H{"error": err.Error(), "code": statusCodeFromSearchError(err)})
		return
	}
	resp.DurationMS = float64(time.Since(start).Milliseconds())
	c.JSON(http.StatusOK, resp)
}

func (h *SearchHandler) Rebuild(c *gin.Context) {
	h.mu.Lock()
	if !h.lastRun.IsZero() && time.Since(h.lastRun) < h.rateLimit {
		h.mu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limited", "code": 429})
		return
	}
	h.lastRun = time.Now()
	h.mu.Unlock()

	go func() {
		if err := h.rebuild.Rebuild(context.Background()); err != nil {
			// 错误通过日志记录，不阻塞响应
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "rebuild_started"})
}

func statusFromSearchError(err error) int {
	switch {
	case errors.Is(err, search.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, search.ErrTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, search.ErrExternalService):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func statusCodeFromSearchError(err error) int {
	switch {
	case errors.Is(err, search.ErrInvalidRequest):
		return 400
	case errors.Is(err, search.ErrTimeout):
		return 504
	case errors.Is(err, search.ErrExternalService):
		return 502
	default:
		return 500
	}
}
