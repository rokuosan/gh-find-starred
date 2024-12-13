package github

import (
	"encoding/json"
	"os"
	"time"
)

type CacheData struct {
	ExpiresAt string        `json:"expires_at"`
	CreatedAt string        `json:"created_at"`
	Data      []interface{} `json:"data"`
}

func Cache(path string, data Repositories) error {
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
