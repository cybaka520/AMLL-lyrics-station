package main

import (
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
	api.POST("/synchronous-lyrics", h.Sync)
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
