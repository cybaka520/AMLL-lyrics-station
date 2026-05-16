package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type SyncService interface{ Start() error }

type LyricsHandler struct {
	service   SyncService
	lastRun   time.Time
	mu        sync.Mutex
	rateLimit time.Duration
}

func New(service SyncService, rateLimit time.Duration) *LyricsHandler {
	return &LyricsHandler{service: service, rateLimit: rateLimit}
}

func (h *LyricsHandler) Sync(c *gin.Context) {
	h.mu.Lock()
	if !h.lastRun.IsZero() && time.Since(h.lastRun) < h.rateLimit {
		h.mu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limited", "code": 429})
		return
	}
	h.lastRun = time.Now()
	h.mu.Unlock()
	go func() { _ = h.service.Start() }()
	c.JSON(http.StatusAccepted, gin.H{"status": "sync_started"})
}
