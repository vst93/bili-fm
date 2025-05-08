package dkv

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

type KVDB struct {
	dbname   string
	readonly bool
}

func __ensurePath(d string) error {
	if fi, err := os.Stat(d); err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(d, os.ModeDir|os.ModePerm); err != nil {
				return err
			}
		}
	} else if !fi.IsDir() {
		return errors.New("File exists is not directory")
	}
	return nil
}

func (p *KVDB) Set(k string, v interface{}) error {
	if p.readonly {
		return errors.New("readonly")
	}
	if err := __ensurePath(p.dbname); err != nil {
		return err
	}
	js, err := json.Marshal([2]interface{}{k, v})
	if err != nil {
		return err
	}
	ioutil.WriteFile(p.dbname+"/"+p.__hash(k), js, os.ModePerm)
	return nil
}

func (p *KVDB) __hash(k string) string {
	h := md5.New()
	h.Write([]byte(k))
	return hex.EncodeToString(h.Sum(nil)[4:12])
}

func (p *KVDB) Get(k string) (ret interface{}) {
	if data, err := ioutil.ReadFile(p.dbname + "/" + p.__hash(k)); err == nil {
		var v [2]interface{}
		if json.Unmarshal(data, &v) == nil {
			return v[1]
		}
	}
	return
}

func (p *KVDB) Interate(callback func(k string, v interface{})) {
	if files, err := ioutil.ReadDir(p.dbname); err == nil {
		for _, fi := range files {
			if fi.IsDir() && fi.Size() > 0 {
				continue
			}
			if data, err := ioutil.ReadFile(p.dbname + "/" + fi.Name()); err == nil {
				var v [2]interface{}
				if json.Unmarshal(data, &v) == nil {
					if k, ok := v[0].(string); ok {
						callback(k, v[1])
					}
				}
			}
		}
	}
}

func (p *KVDB) Del(k string) {
	if !p.readonly {
		os.Remove(p.dbname + "/" + p.__hash(k))
	}
}

func (p *KVDB) Cls() error {
	if p.readonly {
		return errors.New("readonly")
	}
	return os.RemoveAll(p.dbname)
}

func (p *KVDB) Close() {
}

func Open(dbname string, readonly bool) (*KVDB, error) {
	absPath, err := filepath.Abs(dbname)
	if err != nil {
		return nil, err
	}
	p := &KVDB{}
	if !readonly {
		if err := __ensurePath(absPath); err != nil {
			return nil, err
		}
	} else {
		if fi, err := os.Stat(absPath); err != nil {
			return nil, err
		} else {
			if !fi.IsDir() {
				return nil, errors.New("directory exists but not a directory")
			}
		}
	}

	p.readonly = readonly
	p.dbname = absPath
	return p, nil
}
