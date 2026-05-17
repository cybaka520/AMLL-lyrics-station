package search

import "go.uber.org/zap"

func logSyncSkip(log Logger, path, reason string) {
	if log != nil {
		log.Warn("sync skipped", zap.String("path", path), zap.String("reason", reason))
	}
}

func logSyncChange(log Logger, action, path string) {
	if log != nil {
		log.Info("sync change", zap.String("action", action), zap.String("path", path))
	}
}
