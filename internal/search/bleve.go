package search

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/rokuosan/gh-find-starred/internal/cache"
	"github.com/rokuosan/gh-find-starred/internal/domain"
)

// CurrentSchemaVersion identifies the on-disk Bleve mapping & metadata schema.
const CurrentSchemaVersion = 1

const indexBatchSize = 200

// IndexMeta describes the active persisted Bleve index directory and schema.
type IndexMeta struct {
	SchemaVersion int    `json:"schema_version"`
	IndexDir      string `json:"index_dir"`
	RepoCount     int    `json:"repo_count"`
}

// DiskStoreOption configures a DiskStore.
type DiskStoreOption func(*DiskStore)

// WithClock overrides the clock used for generating unique index directory names.
func WithClock(fn func() time.Time) DiskStoreOption {
	return func(s *DiskStore) {
		if fn != nil {
			s.now = fn
		}
	}
}

// DiskStore manages a persistent on-disk Bleve index paired with TTL metadata.
type DiskStore struct {
	dir       string
	metaStore cache.Store[IndexMeta]
	now       func() time.Time
}

// NewDiskStore constructs a DiskStore rooted at dir using metaStore for TTL metadata.
func NewDiskStore(dir string, metaStore cache.Store[IndexMeta], opts ...DiskStoreOption) *DiskStore {
	s := &DiskStore{
		dir:       dir,
		metaStore: metaStore,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Load validates the cached metadata and on-disk Bleve index, returning a read-only
// IndexSearcher and the cached repository count.
func (s *DiskStore) Load(ctx context.Context) (domain.IndexSearcher, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if s.metaStore == nil {
		return nil, 0, cache.ErrCacheMiss
	}

	meta, err := s.metaStore.Get()
	if err != nil {
		return nil, 0, err
	}
	if meta.SchemaVersion != CurrentSchemaVersion {
		return nil, 0, fmt.Errorf("unsupported index schema version: %d", meta.SchemaVersion)
	}
	if !isSafeIndexDirName(meta.IndexDir) {
		return nil, 0, fmt.Errorf("invalid index directory name: %q", meta.IndexDir)
	}

	indexPath := filepath.Join(s.dir, meta.IndexDir)
	idx, err := openReadOnly(indexPath)
	if err != nil {
		return nil, 0, fmt.Errorf("open persisted index: %w", err)
	}
	if _, err := idx.DocCount(); err != nil {
		_ = idx.Close()
		return nil, 0, fmt.Errorf("validate persisted index: %w", err)
	}
	if err := idx.Close(); err != nil {
		return nil, 0, fmt.Errorf("close validated index: %w", err)
	}

	return &diskIndex{path: indexPath}, meta.RepoCount, nil
}

// Save builds a fresh on-disk Bleve index in a unique directory, commits metadata
// atomically, removes stale index directories, and returns a read-only IndexSearcher.
func (s *DiskStore) Save(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.metaStore == nil {
		return nil, errors.New("metadata store is not configured")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create index root directory: %w", err)
	}

	indexDirName := fmt.Sprintf("index-%d-%s.bleve", s.now().UTC().UnixNano(), randomHex(4))
	indexPath := filepath.Join(s.dir, indexDirName)

	idx, err := bleve.New(indexPath, newRepositoryMapping())
	if err != nil {
		return nil, fmt.Errorf("create bleve disk index: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(indexPath)
		}
	}()

	if err := indexRepositories(ctx, idx, repos); err != nil {
		_ = idx.Close()
		return nil, err
	}
	if err := idx.Close(); err != nil {
		return nil, fmt.Errorf("close bleve disk index: %w", err)
	}

	meta := IndexMeta{
		SchemaVersion: CurrentSchemaVersion,
		IndexDir:      indexDirName,
		RepoCount:     len(repos),
	}
	if err := s.metaStore.Set(meta); err != nil {
		return nil, err
	}

	committed = true
	removeStaleIndexes(s.dir, indexDirName)

	return &diskIndex{path: indexPath}, nil
}

// Fallback returns an in-memory IndexSearcher when persisting to disk fails.
func (s *DiskStore) Fallback(repos []domain.Repository) domain.IndexSearcher {
	return &memoryIndex{repos: repos}
}

type diskIndex struct {
	path string
}

func (d *diskIndex) Search(ctx context.Context, query string, limit int) ([]domain.SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	idx, err := openReadOnly(d.path)
	if err != nil {
		return nil, fmt.Errorf("open persisted index for search: %w", err)
	}
	defer idx.Close()

	return executeSearch(ctx, idx, query, limit)
}

type memoryIndex struct {
	repos []domain.Repository
}

func (m *memoryIndex) Search(ctx context.Context, query string, limit int) ([]domain.SearchResult, error) {
	if len(m.repos) == 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	idx, err := bleve.NewMemOnly(newRepositoryMapping())
	if err != nil {
		return nil, fmt.Errorf("create bleve memory index: %w", err)
	}
	defer idx.Close()

	if err := indexRepositories(ctx, idx, m.repos); err != nil {
		return nil, err
	}

	return executeSearch(ctx, idx, query, limit)
}

func newRepositoryMapping() mapping.IndexMapping {
	docMapping := bleve.NewDocumentMapping()
	docMapping.Dynamic = false

	storedText := bleve.NewTextFieldMapping()
	storedText.Store = true
	storedText.Index = true

	indexedOnlyText := bleve.NewTextFieldMapping()
	indexedOnlyText.Store = false
	indexedOnlyText.Index = true

	docMapping.AddFieldMappingsAt("name", storedText)
	docMapping.AddFieldMappingsAt("url", storedText)
	docMapping.AddFieldMappingsAt("description", storedText)
	docMapping.AddFieldMappingsAt("readme", indexedOnlyText)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	return indexMapping
}

func indexRepositories(ctx context.Context, idx bleve.Index, repos []domain.Repository) error {
	for start := 0; start < len(repos); start += indexBatchSize {
		if err := ctx.Err(); err != nil {
			return err
		}

		end := start + indexBatchSize
		if end > len(repos) {
			end = len(repos)
		}

		batch := idx.NewBatch()
		for i := start; i < end; i++ {
			repo := repos[i]
			id := repo.URL
			if id == "" {
				id = fmt.Sprintf("%s#%d", repo.Name, i)
			}
			if err := batch.Index(id, repo); err != nil {
				return fmt.Errorf("stage repository %q: %w", repo.Name, err)
			}
		}

		if err := idx.Batch(batch); err != nil {
			return fmt.Errorf("commit repository batch: %w", err)
		}
	}
	return nil
}

func openReadOnly(path string) (bleve.Index, error) {
	return bleve.OpenUsing(path, map[string]interface{}{
		"read_only": true,
	})
}

func executeSearch(ctx context.Context, idx bleve.Index, query string, limit int) ([]domain.SearchResult, error) {
	docCount, err := idx.DocCount()
	if err != nil {
		return nil, fmt.Errorf("count indexed documents: %w", err)
	}
	if docCount == 0 {
		return nil, nil
	}

	size := limit
	if size <= 0 {
		if docCount > uint64(math.MaxInt) {
			size = math.MaxInt
		} else {
			size = int(docCount)
		}
	}

	q := bleve.NewQueryStringQuery(query)
	req := bleve.NewSearchRequestOptions(q, size, 0, false)
	req.Fields = []string{"name", "url", "description"}

	res, err := idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute search: %w", err)
	}

	results := make([]domain.SearchResult, 0, len(res.Hits))
	for _, hit := range res.Hits {
		results = append(results, domain.SearchResult{
			Repository: domain.Repository{
				Name:        stringField(hit.Fields, "name"),
				URL:         stringField(hit.Fields, "url"),
				Description: stringField(hit.Fields, "description"),
			},
			Score: hit.Score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Repository.Name != results[j].Repository.Name {
			return results[i].Repository.Name < results[j].Repository.Name
		}
		return results[i].Repository.URL < results[j].Repository.URL
	})

	return results, nil
}

func stringField(fields map[string]interface{}, key string) string {
	if fields == nil {
		return ""
	}
	val, ok := fields[key]
	if !ok {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}

func isSafeIndexDirName(name string) bool {
	if name == "" || filepath.Base(name) != name {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	return strings.HasPrefix(name, "index-") && strings.HasSuffix(name, ".bleve")
}

func removeStaleIndexes(dir, activeDir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == activeDir {
			continue
		}
		if isSafeIndexDirName(name) {
			_ = os.RemoveAll(filepath.Join(dir, name))
		}
	}
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}
