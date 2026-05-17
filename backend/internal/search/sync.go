package search

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/meilisearch/meilisearch-go"
	ttml "github.com/xiaowumin-mark/amll-ttml"
	"go.uber.org/zap"
)

type SyncService struct {
	client *Client
	cfg    Config
	state  map[string]fileState
	entries map[string]indexEntry
	mu     sync.RWMutex
	log    Logger
}

type Config struct {
	IndexPath string
	LyricsDir string
	BatchSize int
	Watcher   bool
}

type fileState struct {
	ModTime     time.Time
	ContentHash string
}

type indexEntry struct {
	Metadata     [][]any
	RawLyricFile string
}

func NewSyncService(client *Client, cfg Config) *SyncService {
	return &SyncService{
		client:  client,
		cfg:     cfg,
		state:   map[string]fileState{},
		entries: map[string]indexEntry{},
	}
}

func (s *SyncService) WithLogger(log Logger) *SyncService {
	s.log = log
	return s
}

func (s *SyncService) Validate() error {
	if s.client == nil || s.cfg.IndexPath == "" || s.cfg.LyricsDir == "" || s.cfg.BatchSize <= 0 {
		return errors.New("sync config is incomplete")
	}
	return nil
}

func (s *SyncService) Rebuild(ctx context.Context) error {
	if err := s.client.EnsureIndex(ctx); err != nil {
		return classifyError(err)
	}
	if err := s.applySettings(); err != nil {
		return classifyError(err)
	}

	entries, err := s.readIndexEntries()
	if err != nil {
		return classifyError(err)
	}

	docs := make([]LyricDocument, 0, len(entries))
	nextState := make(map[string]fileState, len(entries))
	s.mu.Lock()
	s.entries = entries
	s.mu.Unlock()

	for rawFile, entry := range entries {
		doc, st, err := s.buildFromEntry(entry)
		if err != nil {
			if s.log != nil {
				s.log.Warn("skip lyric file", zap.String("file", rawFile), zap.Error(err))
			}
			continue
		}
		docs = append(docs, doc)
		nextState[rawFile] = st
	}

	if err := s.pushDocuments(ctx, docs); err != nil {
		return classifyError(err)
	}
	s.state = nextState
	if s.log != nil {
		s.log.Info("search rebuild done", zap.Int("documents", len(docs)), zap.Int("tracked", len(nextState)))
	}
	return nil
}

func (s *SyncService) StartWatcher(ctx context.Context) error {
	if !s.cfg.Watcher {
		return nil
	}
	return Watch(ctx, []string{s.cfg.IndexPath, s.cfg.LyricsDir}, func(evt WatchEvent) error {
		if s.log != nil {
			s.log.Info("watcher triggered", zap.String("source", evt.Source), zap.String("name", evt.Name))
		}
		return s.Sync(ctx)
	})
}

func (s *SyncService) Sync(ctx context.Context) error {
	current, err := s.snapshot()
	if err != nil {
		return classifyError(err)
	}
	changed, deleted := diffStates(s.state, current)
	if len(changed) == 0 && len(deleted) == 0 {
		return nil
	}
	return s.IncrementalSync(ctx, changed, deleted)
}

func (s *SyncService) IncrementalSync(ctx context.Context, changedFiles []string, deletedFiles []string) error {
	docs := make([]LyricDocument, 0, len(changedFiles))
	updated := make(map[string]fileState, len(changedFiles))

	for _, rel := range changedFiles {
		entry, ok := s.lookupEntry(rel)
		if !ok {
			if s.log != nil {
				s.log.Warn("skip changed file", zap.String("file", rel), zap.String("reason", "entry not found in cache"))
			}
			continue
		}
		doc, st, err := s.buildFromEntry(entry)
		if err != nil {
			if s.log != nil {
				s.log.Warn("build failed", zap.String("file", rel), zap.Error(err))
			}
			continue
		}
		docs = append(docs, doc)
		updated[rel] = st
	}

	if err := s.pushDocuments(ctx, docs); err != nil {
		return classifyError(err)
	}
	if len(deletedFiles) > 0 {
		if _, err := s.client.Index().DeleteDocuments(deletedFiles, nil); err != nil {
			return classifyError(err)
		}
	}

	for k, v := range updated {
		s.state[k] = v
	}
	for _, k := range deletedFiles {
		delete(s.state, k)
		delete(s.entries, k)
	}

	if s.log != nil {
		s.log.Info("incremental sync done",
			zap.Int("changed", len(changedFiles)),
			zap.Int("deleted", len(deletedFiles)),
			zap.Int("upserted", len(docs)),
		)
	}
	return nil
}

func (s *SyncService) applySettings() error {
	_, err := applyIndexSettings(s.client.Index())
	return err
}

func (s *SyncService) pushDocuments(ctx context.Context, docs []LyricDocument) error {
	if len(docs) == 0 {
		return nil
	}
	for start := 0; start < len(docs); start += s.cfg.BatchSize {
		end := start + s.cfg.BatchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[start:end]
		pk := "id"
		res, err := s.client.Index().AddDocuments(batch, &meilisearch.DocumentOptions{PrimaryKey: &pk})
		if err != nil {
			return err
		}
		if err := s.client.WaitTask(ctx, res.TaskUID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SyncService) readIndexEntries() (map[string]indexEntry, error) {
	f, err := os.Open(s.cfg.IndexPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]indexEntry)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry, err := decodeIndexEntry(scanner.Bytes())
		if err != nil {
			continue
		}
		entries[entry.RawLyricFile] = entry
	}
	return entries, scanner.Err()
}

func (s *SyncService) lookupEntry(rawFile string) (indexEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[rawFile]
	return entry, ok
}

func (s *SyncService) buildFromEntry(entry indexEntry) (LyricDocument, fileState, error) {
	return s.buildDocument(entry.RawLyricFile, entry.Metadata)
}

func (s *SyncService) buildDocument(rawFile string, metadata [][]any) (LyricDocument, fileState, error) {
	full := filepath.Join(s.cfg.LyricsDir, rawFile)
	info, err := os.Stat(full)
	if err != nil {
		return LyricDocument{}, fileState{}, err
	}
	content, err := s.readLyricFile(full)
	if err != nil {
		return LyricDocument{}, fileState{}, err
	}

	doc := LyricDocument{
		ID:           rawFile,
		RawLyricFile: rawFile,
		UpdatedAt:    info.ModTime().Unix(),
		LyricContent: string(content),
	}
	for _, kv := range metadata {
		if len(kv) != 2 {
			continue
		}
		key, _ := kv[0].(string)
		switch key {
		case "musicName":
			doc.MusicNames = toStrings(kv[1])
		case "artists":
			doc.Artists = toStrings(kv[1])
		case "album":
			doc.Albums = toStrings(kv[1])
		case "ncmMusicId":
			doc.NcmMusicIds = toStrings(kv[1])
		case "qqMusicId":
			doc.QqMusicIds = toStrings(kv[1])
		case "spotifyId":
			doc.SpotifyIds = toStrings(kv[1])
		case "appleMusicId":
			doc.AppleMusicIds = toStrings(kv[1])
		case "isrc":
			doc.Isrcs = toStrings(kv[1])
		case "ttmlAuthorGithub":
			doc.TtmlAuthorGithub = firstString(kv[1])
		case "ttmlAuthorGithubLogin":
			doc.TtmlAuthorLogin = firstString(kv[1])
		}
	}

	parsed, err := ttml.ParseLyric(string(content))
	if err == nil {
		doc.TranslatedLyric = collectTranslated(parsed)
		doc.RomanLyric = collectRoman(parsed)
	} else if s.log != nil {
		s.log.Warn("ttml parse failed", zap.String("file", rawFile), zap.Error(err))
	}

	return doc, fileState{ModTime: info.ModTime(), ContentHash: fileHash(content)}, nil
}

func (s *SyncService) snapshot() (map[string]fileState, error) {
	s.mu.RLock()
	entries := make(map[string]indexEntry, len(s.entries))
	for k, v := range s.entries {
		entries[k] = v
	}
	s.mu.RUnlock()
	if len(entries) == 0 {
		loaded, err := s.readIndexEntries()
		if err != nil {
			return nil, err
		}
		entries = loaded
		s.mu.Lock()
		s.entries = loaded
		s.mu.Unlock()
	}
	state := make(map[string]fileState, len(entries))
	for rawFile := range entries {
		full := filepath.Join(s.cfg.LyricsDir, rawFile)
		content, info, err := s.readLyricFileWithInfo(full)
		if err != nil {
			if s.log != nil {
				s.log.Warn("snapshot skip", zap.String("file", rawFile), zap.Error(err))
			}
			continue
		}
		state[rawFile] = fileState{ModTime: info.ModTime(), ContentHash: fileHash(content)}
	}
	return state, nil
}

func (s *SyncService) readLyricFile(full string) ([]byte, error) {
	content, _, err := s.readLyricFileWithInfo(full)
	return content, err
}

func (s *SyncService) readLyricFileWithInfo(full string) ([]byte, os.FileInfo, error) {
	info, err := os.Stat(full)
	if err != nil {
		return nil, nil, err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, nil, err
	}
	return content, info, nil
}

func decodeIndexEntry(line []byte) (indexEntry, error) {
	var raw struct {
		Metadata     [][]any `json:"metadata"`
		RawLyricFile string  `json:"rawLyricFile"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return indexEntry{}, err
	}
	if raw.RawLyricFile == "" {
		return indexEntry{}, errors.New("missing rawLyricFile")
	}
	return indexEntry{Metadata: raw.Metadata, RawLyricFile: raw.RawLyricFile}, nil
}

func diffStates(prev, current map[string]fileState) (changed, deleted []string) {
	for path, cur := range current {
		old, ok := prev[path]
		if !ok || old.ContentHash != cur.ContentHash || !old.ModTime.Equal(cur.ModTime) {
			changed = append(changed, path)
		}
	}
	for path := range prev {
		if _, ok := current[path]; !ok {
			deleted = append(deleted, path)
		}
	}
	sort.Strings(changed)
	sort.Strings(deleted)
	return changed, deleted
}

func fileHash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func collectTranslated(l ttml.TTMLLyric) string {
	parts := make([]string, 0, len(l.LyricLines))
	for _, line := range l.LyricLines {
		if strings.TrimSpace(line.TranslatedLyric) != "" {
			parts = append(parts, line.TranslatedLyric)
		}
	}
	return strings.Join(parts, "\n")
}

func collectRoman(l ttml.TTMLLyric) string {
	parts := make([]string, 0, len(l.LyricLines))
	for _, line := range l.LyricLines {
		if strings.TrimSpace(line.RomanLyric) != "" {
			parts = append(parts, line.RomanLyric)
		}
	}
	return strings.Join(parts, "\n")
}

func toStrings(v any) []string {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(a))
	for _, item := range a {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func firstString(v any) string {
	vals := toStrings(v)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}
