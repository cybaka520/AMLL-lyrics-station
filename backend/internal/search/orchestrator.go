package search

import (
	"context"
	"path/filepath"

	"go.uber.org/zap"
)

type Orchestrator struct {
	Search *Service
	Sync   *SyncService
	Log    Logger
}

func NewOrchestrator(client *Client, cfg Config, log Logger) *Orchestrator {
	syncSvc := NewSyncService(client, cfg).WithLogger(log)
	return &Orchestrator{Search: NewService(client), Sync: syncSvc, Log: log}
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.Sync.Validate(); err != nil { return err }
	if err := o.Search.Initialize(ctx); err != nil && o.Log != nil { o.Log.Warn("search initialize failed", zap.Error(err)) }
	if err := o.Sync.Rebuild(ctx); err != nil && o.Log != nil { o.Log.Warn("search rebuild failed", zap.Error(err)) }
	if o.Sync.cfg.Watcher { if o.Log != nil { o.Log.Info("search watcher started", zap.String("index_path", o.Sync.cfg.IndexPath), zap.String("lyrics_dir", filepath.Clean(o.Sync.cfg.LyricsDir))) }; return o.Sync.StartWatcher(ctx) }
	return nil
}
