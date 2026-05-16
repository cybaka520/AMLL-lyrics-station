package lyrics

import (
	"fmt"
	"path/filepath"
	"time"

	"amllhub/backend/internal/db"
	gitrepo "amllhub/backend/internal/git"
	"amllhub/backend/internal/model"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

const batchSize = 500

type Service struct {
	DB      *sqlx.DB
	RepoURL string
	Root    string
	Logger  *zap.Logger
}

type SyncResult struct {
	Files   int
	Deleted int
	Started time.Time
	Ended   time.Time
}

type LyricsError struct {
	Stage string
	Err   error
}

func (e *LyricsError) Error() string { return fmt.Sprintf("%s: %v", e.Stage, e.Err) }
func (e *LyricsError) Unwrap() error { return e.Err }

func (s *Service) Start() error {
	started := time.Now().UTC()
	if s.Logger != nil {
		s.Logger.Info("同步开始", zap.Time("started_at", started))
	}
	if _, err := gitrepo.EnsureRepo(s.RepoURL, s.Root); err != nil {
		return s.fail("git", err, started, 0, 0)
	}
	oldCommit, newCommit, err := gitrepo.Pull(s.Root)
	if err != nil {
		return s.fail("git", err, started, 0, 0)
	}
	if oldCommit == newCommit {
		if s.Logger != nil {
			s.Logger.Info("同步完成", zap.Time("started_at", started), zap.Time("ended_at", time.Now().UTC()), zap.Int64("duration_ms", 0), zap.Int("files", 0), zap.Int("deleted", 0), zap.String("reason", "no_changes"))
		}
		return nil
	}
	changes, err := gitrepo.Diff(s.Root, oldCommit, newCommit)
	if err != nil {
		return s.fail("git", err, started, 0, 0)
	}
	if s.Logger != nil {
		s.Logger.Info("检测到变更", zap.Int("change_files", len(changes)), zap.String("old_commit", oldCommit), zap.String("new_commit", newCommit))
	}
	if len(changes) == 0 {
		if s.Logger != nil {
			s.Logger.Info("同步完成", zap.Time("started_at", started), zap.Time("ended_at", time.Now().UTC()), zap.Int64("duration_ms", time.Since(started).Milliseconds()), zap.Int("files", 0), zap.Int("deleted", 0), zap.String("reason", "empty_diff"))
		}
		return nil
	}
	var result SyncResult
	err = db.WithTx(s.DB, func(tx *sqlx.Tx) error {
		deleted := make([]string, 0)
		upserts := make([]model.FileMeta, 0, len(changes))
		for _, rel := range changes {
			full := filepath.Join(s.Root, rel)
			ok, err := gitrepo.FileExistsAtCommit(s.Root, newCommit, rel)
			if err != nil {
				return &LyricsError{Stage: "git", Err: err}
			}
			if !ok {
				deleted = append(deleted, filepath.ToSlash(rel))
				continue
			}
			meta, err := ScanOne(s.Root, full)
			if err != nil {
				return &LyricsError{Stage: "fs", Err: err}
			}
			upserts = append(upserts, meta)
		}
		if err := deleteBatch(tx, deleted); err != nil {
			return err
		}
		if err := upsertBatch(tx, upserts); err != nil {
			return err
		}
		result.Files = len(upserts)
		result.Deleted = len(deleted)
		return nil
	})
	result.Started = started
	result.Ended = time.Now().UTC()
	if err != nil {
		return s.fail("db", err, result.Started, result.Files, result.Deleted)
	}
	if s.Logger != nil {
		s.Logger.Info("同步完成", zap.Time("started_at", result.Started), zap.Time("ended_at", result.Ended), zap.Int64("duration_ms", result.Ended.Sub(result.Started).Milliseconds()), zap.Int("files", result.Files), zap.Int("deleted", result.Deleted))
	}
	return nil
}

func (s *Service) fail(stage string, err error, started time.Time, files, deleted int) error {
	wrapped := &LyricsError{Stage: stage, Err: err}
	if s.Logger != nil {
		s.Logger.Error("同步失败", zap.String("stage", stage), zap.Time("started_at", started), zap.Int64("duration_ms", time.Since(started).Milliseconds()), zap.Int("files", files), zap.Int("deleted", deleted), zap.String("error", err.Error()))
	}
	return wrapped
}

func upsertBatch(tx *sqlx.Tx, metas []model.FileMeta) error {
	for start := 0; start < len(metas); start += batchSize {
		end := start + batchSize
		if end > len(metas) {
			end = len(metas)
		}
		query, args := buildUpsertQuery(metas[start:end])
		if query == "" {
			continue
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	return nil
}

func buildUpsertQuery(metas []model.FileMeta) (string, []any) {
	if len(metas) == 0 {
		return "", nil
	}
	query := "INSERT INTO lyrics_schema.lyrics (file_path, content_hash, modified_at, file_content, updated_at) VALUES "
	args := make([]any, 0, len(metas)*4)
	for i, meta := range metas {
		if i > 0 {
			query += ","
		}
		base := i*4 + 1
		query += fmt.Sprintf("($%d,$%d,$%d,$%d,NOW())", base, base+1, base+2, base+3)
		args = append(args, filepath.ToSlash(meta.Path), meta.ContentHash, meta.ModifiedAt, meta.Content)
	}
	query += " ON CONFLICT (file_path) DO UPDATE SET content_hash = EXCLUDED.content_hash, modified_at = EXCLUDED.modified_at, file_content = EXCLUDED.file_content, updated_at = NOW()"
	return query, args
}

func deleteBatch(tx *sqlx.Tx, paths []string) error {
	for start := 0; start < len(paths); start += batchSize {
		end := start + batchSize
		if end > len(paths) {
			end = len(paths)
		}
		chunk := paths[start:end]
		if len(chunk) == 0 {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM lyrics_schema.lyrics WHERE file_path = ANY($1)`, pq.StringArray(chunk)); err != nil {
			return &LyricsError{Stage: "db", Err: err}
		}
	}
	return nil
}
