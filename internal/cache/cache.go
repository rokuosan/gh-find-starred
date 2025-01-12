package cache

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/rokuosan/gh-find-starred/pkg/api"
)

type CacheController interface {
	Get(key string) (interface{}, error)
	Cache(key string, data interface{}) error
}

type PeriodicalCache struct {
	Period time.Duration
	Path   string
}

type PeriodicalCacheData struct {
	ExpiresAt string        `json:"expires_at"`
	CreatedAt string        `json:"created_at"`
	Data      []interface{} `json:"data"`
}

func NewPeriodicalCache(path string, period time.Duration) *PeriodicalCache {
	return &PeriodicalCache{
		Period: period,
		Path:   path,
	}
}

func (c *PeriodicalCache) Get(key string) (interface{}, error) {

	return nil, nil
}

func (c *PeriodicalCache) Cache(key string, data interface{}) error {
	now := time.Now().Format(time.RFC3339)
	expiresAt := time.Now().Add(c.Period).Format(time.RFC3339)
	cacheData := PeriodicalCacheData{
		ExpiresAt: expiresAt,
		CreatedAt: now,
		Data:      []interface{}{data},
	}

	jsonData, err := c.toJSON(cacheData)
	if err != nil {
		return err
	}

	file, err := os.Create(c.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	return c.write(jsonData, file)
}

func (c *PeriodicalCache) toJSON(data PeriodicalCacheData) ([]byte, error) {
	buffer := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buffer)
	err := encoder.Encode(data)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (c *PeriodicalCache) write(data []byte, writer io.Writer) error {
	_, err := writer.Write(data)
	return err
}

type CacheData struct {
	ExpiresAt string        `json:"expires_at"`
	CreatedAt string        `json:"created_at"`
	Data      []interface{} `json:"data"`
}

func Cache(path string, data api.GitHubRepositories) error {
	now := time.Now().Format(time.RFC3339)
	expiresAt := time.Now().Add(time.Hour * 24).Format(time.RFC3339)
	cacheData := CacheData{
		ExpiresAt: expiresAt,
		CreatedAt: now,
		Data:      make([]interface{}, len(data)),
	}
	for i, d := range data {
		cacheData.Data[i] = d
	}

	// ファイルを作成
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// ファイルに書き込む
	enc := json.NewEncoder(f)
	if err := enc.Encode(cacheData); err != nil {
		return err
	}

	return nil
}
