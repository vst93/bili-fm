// Package store 是 bili-FM 的本地键值存储。
//
// 格式与旧版完全一致，这样从 Wails / Tauri 版本升级上来的用户，登录态
// （SESSDATA / uname / face / mid）和歌单（playlist / playlist_play_mode）
// 都能直接读到，不需要重新登录：
//
//	<os.UserConfigDir()>/bili-fm/data.db/     一个目录
//	<hex(md5(key)[4:12])>                     每个 key 一个文件，无扩展名
//	["key", value]                            文件内容，JSON 数组
//
// 这是 Go 版 dkv（service/dkv/dkv.go）的忠实移植，Rust 版
// （src-tauri/src/dkv.rs）读写的也是同一格式。
package store

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// KV 是一个键值库，对应旧版的 dkv.KVDB。
type KV struct {
	dir      string
	readonly bool

	mu sync.Mutex
}

// Dir 返回旧版数据目录：<os.UserConfigDir()>/bili-fm/data.db。
func Dir() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "bili-fm", "data.db"), nil
}

// Open 打开（必要时创建）数据库。readonly 为真时不写入。
func Open(dir string, readonly bool) (*KV, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if !readonly {
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return nil, err
		}
	} else if fi, err := os.Stat(abs); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, errors.New("store: 路径存在但不是目录")
	}
	return &KV{dir: abs, readonly: readonly}, nil
}

// OpenDefault 打开默认位置。
func OpenDefault() (*KV, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	return Open(dir, false)
}

// hash 复刻 Go 版 dkv 的取名方式：md5 摘要的第 4..12 个字节，hex 编码。
func hash(key string) string {
	sum := md5.Sum([]byte(key))
	return hex.EncodeToString(sum[4:12])
}

func (k *KV) path(key string) string {
	return filepath.Join(k.dir, hash(key))
}

// Set 写入一个键。值必须是可 JSON 序列化的。
func (k *KV) Set(key string, value any) error {
	if k.readonly {
		return errors.New("store: 只读")
	}
	data, err := json.Marshal([2]any{key, value})
	if err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := os.MkdirAll(k.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(k.path(key), data, 0o644)
}

// Get 读取一个键，不存在时返回 nil。
func (k *KV) Get(key string) any {
	data, err := os.ReadFile(k.path(key))
	if err != nil {
		return nil
	}
	var pair [2]any
	if json.Unmarshal(data, &pair) != nil {
		return nil
	}
	return pair[1]
}

// String 读取字符串键，缺失或类型不符时返回 ""。
func (k *KV) String(key string) string {
	if s, ok := k.Get(key).(string); ok {
		return s
	}
	return ""
}

// SetString 写入字符串键。
func (k *KV) SetString(key, value string) error { return k.Set(key, value) }

// Del 删除一个键。
func (k *KV) Del(key string) {
	if k.readonly {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	_ = os.Remove(k.path(key))
}

// Iterate 遍历所有键（顺序不保证），用于迁移或诊断。
func (k *KV) Iterate(fn func(key string, value any)) {
	entries, err := os.ReadDir(k.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(k.dir, e.Name()))
		if err != nil {
			continue
		}
		var pair [2]any
		if json.Unmarshal(data, &pair) != nil {
			continue
		}
		if key, ok := pair[0].(string); ok {
			fn(key, pair[1])
		}
	}
}

// Path 返回数据库目录，用于日志与诊断。
func (k *KV) Path() string { return k.dir }

// Describe 返回一行人类可读的状态，用于启动日志。
func (k *KV) Describe() string {
	return fmt.Sprintf("store %s", k.dir)
}
