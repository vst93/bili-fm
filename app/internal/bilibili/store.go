package bilibili

import "github.com/vst93/bili-fm/app/internal/store"

// kv 是包级的本地存储句柄。旧版 dkv 也是包级单例（service/dkv），
// api.go 里几十处 GetItem/SetItem 直接沿用这个形状，移植时不用改调用点。
var kv *store.KV

// UseStore 在启动时绑定存储，之后 GetItem/SetItem 才能用。
func UseStore(s *store.KV) { kv = s }

// GetItem 读取一个键，未绑定存储或不存在时返回 nil。
func GetItem(key string) any {
	if kv == nil {
		return nil
	}
	return kv.Get(key)
}

// SetItem 写入一个键，未绑定存储时静默忽略（只读场景）。
func SetItem(key string, value any) {
	if kv == nil {
		return
	}
	_ = kv.Set(key, value)
}
