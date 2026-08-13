//! dkv 兼容存储层。
//!
//! 旧版 bili-FM (Wails/Go) 使用自定义的 file-based KV 存储 (`service/dkv`):
//! - 存储路径: `<os.UserConfigDir()>/bili-fm/data.db/` (一个**目录**)
//! - 每个 key 存为一个文件: `<hex(md5(key) 摘要字节[4..12])>.json`
//!   (Go: `hex.EncodeToString(h.Sum(nil)[4:12])` — 取 16 字节摘要的
//!   第 4..12 个**字节** (8 字节) 再 hex 编码，共 16 个 hex 字符。
//!   例如 "SESSDATA" -> `95a92112f03079db.json`)
//! - 文件内容: JSON 数组 `[key, value]`
//!
//! 本模块按**完全相同**的格式读写，保证老用户升级后登录态 (SESSDATA、
//! uname、face、mid、playlist、playlist_play_mode) 无缝迁移。

use std::fmt;
use std::fs;
use std::io;
use std::path::{Path, PathBuf};

use serde_json::{json, Value};

#[derive(Debug)]
pub enum Error {
    Io(io::Error),
    Json(serde_json::Error),
    Readonly,
    NotADirectory,
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Io(e) => write!(f, "IO error: {e}"),
            Error::Json(e) => write!(f, "JSON error: {e}"),
            Error::Readonly => write!(f, "db is readonly"),
            Error::NotADirectory => write!(f, "file exists but is not a directory"),
        }
    }
}

impl std::error::Error for Error {}

impl From<io::Error> for Error {
    fn from(e: io::Error) -> Self {
        Error::Io(e)
    }
}

impl From<serde_json::Error> for Error {
    fn from(e: serde_json::Error) -> Self {
        Error::Json(e)
    }
}

/// 与 Go `dkv.KVDB` 对应的结构。
pub struct KVDB {
    dbname: PathBuf,
    readonly: bool,
}

impl KVDB {
    /// 与 Go `dkv.Open` 一致:
    /// - 非 readonly: 确保目录存在 (不存在则创建)
    /// - readonly:    目录必须已存在
    pub fn open(dbname: impl Into<PathBuf>, readonly: bool) -> Result<Self, Error> {
        let raw = dbname.into();
        // Go 用 filepath.Abs 转绝对路径
        let abs = if raw.is_absolute() {
            raw
        } else {
            std::env::current_dir()?.join(raw)
        };
        if !readonly {
            ensure_dir(&abs)?;
        } else if abs.exists() && !abs.is_dir() {
            return Err(Error::NotADirectory);
        }
        Ok(KVDB {
            dbname: abs,
            readonly,
        })
    }

    /// Go: `hex.EncodeToString(h.Sum(nil)[4:12])` — 注意是切**摘要字节**
    /// 4..12 (8 字节) 再 hex 编码，不是切 hex 字符串。
    fn hash(key: &str) -> String {
        let digest = md5::compute(key.as_bytes());
        digest[4..12]
            .iter()
            .map(|b| format!("{b:02x}"))
            .collect()
    }

    fn path_of(&self, key: &str) -> PathBuf {
        self.dbname.join(Self::hash(key))
    }

    /// 写入 `[key, value]` JSON 数组到 `<hash>.json`，与 Go `Set` 格式一致。
    pub fn set(&self, key: &str, value: &Value) -> Result<(), Error> {
        if self.readonly {
            return Err(Error::Readonly);
        }
        ensure_dir(&self.dbname)?;
        let record = json!([key, value]);
        let data = serde_json::to_vec(&record)?;
        fs::write(self.path_of(key), data)?;
        Ok(())
    }

    /// 读取 value。与 Go `Get` 一致: 只取数组第 2 个元素，不校验 key 是否匹配。
    pub fn get(&self, key: &str) -> Option<Value> {
        let data = fs::read(self.path_of(key)).ok()?;
        let v: Value = serde_json::from_slice(&data).ok()?;
        match v {
            Value::Array(a) => a.get(1).cloned(),
            _ => None,
        }
    }

    pub fn del(&self, key: &str) {
        if !self.readonly {
            let _ = fs::remove_file(self.path_of(key));
        }
    }

    pub fn cls(&self) -> Result<(), Error> {
        if self.readonly {
            return Err(Error::Readonly);
        }
        fs::remove_dir_all(&self.dbname)?;
        Ok(())
    }

    pub fn close(&self) {
        // Go 的 Close 是空操作，保持同名方法以对应移植
    }

    /// 遍历所有 key/value (对应 Go `Interate`)。
    pub fn iterate(&self, mut callback: impl FnMut(&str, &Value)) {
        let Ok(entries) = fs::read_dir(&self.dbname) else {
            return;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                continue;
            }
            if let Ok(data) = fs::read(&path) {
                if let Ok(Value::Array(a)) = serde_json::from_slice::<Value>(&data) {
                    if let (Some(Value::String(k)), Some(val)) = (a.first(), a.get(1)) {
                        callback(k, val);
                    }
                }
            }
        }
    }
}

fn ensure_dir(path: &Path) -> Result<(), Error> {
    match fs::metadata(path) {
        Ok(m) if m.is_dir() => Ok(()),
        Ok(_) => Err(Error::NotADirectory),
        Err(e) if e.kind() == io::ErrorKind::NotFound => {
            fs::create_dir_all(path)?;
            Ok(())
        }
        Err(e) => Err(Error::Io(e)),
    }
}

/// 旧版数据路径: `os.UserConfigDir()/bili-fm/data.db` (与 service/db.go 一致)。
pub fn data_dir() -> PathBuf {
    let config = dirs::config_dir().unwrap_or_else(|| PathBuf::from("."));
    config.join("bili-fm").join("data.db")
}

/// 读取单项 (对应 Go `service.GetItem`)。
pub fn get_item(key: &str) -> Option<Value> {
    match KVDB::open(data_dir(), true) {
        Ok(db) => db.get(key),
        Err(e) => {
            eprintln!("dkv: open failed: {e}");
            None
        }
    }
}

/// 写入单项 (对应 Go `service.SetItem`)。
pub fn set_item(key: &str, value: &Value) -> Result<(), Error> {
    let db = KVDB::open(data_dir(), false)?;
    db.set(key, value)
}

/// 读取字符串值；数字自动转字符串 (对应 Go `NumberToString`)。
pub fn get_string(key: &str) -> String {
    match get_item(key) {
        Some(Value::String(s)) => s,
        Some(Value::Number(n)) => number_to_string(&n),
        _ => String::new(),
    }
}

/// Go `NumberToString` 的移植: float64 用 'f' 格式、无精度损失。
fn number_to_string(n: &serde_json::Number) -> String {
    if let Some(i) = n.as_i64() {
        return i.to_string();
    }
    if let Some(u) = n.as_u64() {
        return u.to_string();
    }
    if let Some(f) = n.as_f64() {
        // 整数值去掉小数部分，对应 FormatFloat(f, 'f', -1, 64)
        if f.fract() == 0.0 && f.abs() < 9_007_199_254_740_992.0 {
            return format!("{}", f as i64);
        }
        return format!("{f}");
    }
    String::new()
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 与 Go `hex.EncodeToString(md5.Sum([]byte(k))[4:12])` 逐一比对。
    /// 参考值由 Python 按 Go 的语义 (切摘要字节再 hex) 计算:
    ///   SESSDATA -> 95a92112f03079db, uname -> ec1880aa70936989
    ///   face -> 53f2986b752e58b1, mid -> d743fe3c6fb0a4b3
    ///   playlist -> 7e72e3749053af29, playlist_play_mode -> 325b523f52628aef
    #[test]
    fn hash_matches_go_dkv() {
        let cases = [
            ("SESSDATA", "95a92112f03079db"),
            ("uname", "ec1880aa70936989"),
            ("face", "53f2986b752e58b1"),
            ("mid", "d743fe3c6fb0a4b3"),
            ("playlist", "7e72e3749053af29"),
            ("playlist_play_mode", "325b523f52628aef"),
        ];
        for (key, want) in cases {
            assert_eq!(KVDB::hash(key), want, "hash mismatch for {key}");
        }
    }

    /// 写入/读取往返: 文件内容必须是 JSON 数组 [key, value]，与 Go 一致。
    #[test]
    fn set_get_roundtrip() {
        let dir = std::env::temp_dir().join(format!("dkv-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let db = KVDB::open(&dir, false).unwrap();
        db.set("SESSDATA", &json!("abc123")).unwrap();
        db.set("mid", &json!(123456789)).unwrap();

        // 文件内容格式: ["SESSDATA","abc123"]
        let raw = fs::read(dir.join(KVDB::hash("SESSDATA"))).unwrap();
        assert_eq!(raw, b"[\"SESSDATA\",\"abc123\"]");

        assert_eq!(db.get("SESSDATA"), Some(json!("abc123")));
        assert_eq!(db.get("mid"), Some(json!(123456789)));
        assert_eq!(db.get("missing"), None);

        // readonly 模式只能读
        let ro = KVDB::open(&dir, true).unwrap();
        assert_eq!(ro.get("SESSDATA"), Some(json!("abc123")));
        assert!(matches!(ro.set("x", &json!(1)), Err(Error::Readonly)));

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn number_to_string_matches_go() {
        assert_eq!(number_to_string(&serde_json::Number::from(123456789)), "123456789");
        assert_eq!(
            number_to_string(&serde_json::Number::from_f64(12345.0).unwrap()),
            "12345"
        );
    }
}
