package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"amllhub/backend/internal/api/handler"
	"amllhub/backend/internal/api/middleware"
	"amllhub/backend/internal/config"
	"amllhub/backend/internal/db"
	"amllhub/backend/internal/logger"
	"amllhub/backend/internal/lyrics"
	"amllhub/backend/internal/search"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	logx, err := logger.New(cfg.Log.Level, cfg.Log.Path, cfg.Log.MaxSize, cfg.Log.MaxBackups, cfg.Log.MaxAge)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logx.Sync() }()

	dbx, err := db.Connect(cfg.Database.Host, cfg.Database.Port, cfg.Database.Name, cfg.Database.User, cfg.Database.Password)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.InitSchema(dbx); err != nil {
		log.Fatal(err)
	}
	svc := &lyrics.Service{DB: dbx, RepoURL: cfg.Git.RepoURL, Root: cfg.Git.LocalPath, Logger: logx}
	bootstrapLyrics(svc, cfg.Git.LocalPath, logx)
	searchClient := search.NewClient(cfg.MeiliSearch.Host, cfg.MeiliSearch.APIKey, cfg.MeiliSearch.IndexName, cfg.MeiliSearch.BatchSize)
	searchOrch := search.NewOrchestrator(searchClient, search.Config{IndexPath: filepath.Join(cfg.Git.LocalPath, "raw-lyrics-index.jsonl"), LyricsDir: filepath.Join(cfg.Git.LocalPath, "raw-lyrics"), BatchSize: cfg.MeiliSearch.BatchSize, Watcher: true}, zapAdapter{logx})
	go func() {
		if err := searchOrch.Start(context.Background()); err != nil {
			logx.Error("search orchestrator failed", zap.Error(err))
		}
	}()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logx.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(start)),
		)
	})
	api := r.Group("/api/v1", middleware.Auth(cfg.API.Key))
	h := handler.New(svc, time.Duration(cfg.Sync.RateLimit)*time.Minute)
	sh := handler.NewSearch(searchOrch.Search, searchOrch.Sync, time.Duration(cfg.Sync.RateLimit)*time.Minute)
	api.POST("/synchronous-lyrics", h.Sync)
	api.GET("/search", sh.Search)
	api.POST("/search/rebuild", sh.Rebuild)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func bootstrapLyrics(svc *lyrics.Service, root string, logx *zap.Logger) {
	if repoExists(root) {
		go func() {
			if err := svc.Start(); err != nil {
				logx.Error("启动歌词同步失败", zap.Error(err))
			}
		}()
		return
	}
	go func() {
		if err := svc.Start(); err != nil {
			logx.Error("启动歌词同步失败", zap.Error(err))
		}
	}()
}

func repoExists(root string) bool {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return true
	}
	return false
}

type zapAdapter struct{ l *zap.Logger }

func (z zapAdapter) Info(msg string, fields ...zap.Field)  { z.l.Info(msg, fields...) }
func (z zapAdapter) Warn(msg string, fields ...zap.Field)  { z.l.Warn(msg, fields...) }
func (z zapAdapter) Error(msg string, fields ...zap.Field) { z.l.Error(msg, fields...) }
