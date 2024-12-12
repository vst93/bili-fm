package main

import "changeme/dkv"

// 读取 sqllite 数据库，查询 data 表中
var dbFiletring = "/Users/vst/Downloads/test.json"

func GetItem(key string) (interface{}) {
	
	db, err := dkv.Open(dbFiletring, false)
	if err != nil {
		panic(db)
	}
	defer db.Close()
	// can store any variable that can marshal with json
	// db.Set("Hello", "World")
	ret := db.Get(key)
	return ret
}


func SetItem(key string, value interface{}) error {
	db, err := dkv.Open(dbFiletring, false)
	if err != nil {
		panic(db)
	}
	defer db.Close()
	return db.Set(key, value)
}