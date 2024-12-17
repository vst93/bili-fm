package main

import (
	"changeme/dkv"
	"fmt"
	"os"
)

// 读取 sqllite 数据库，查询 data 表中
var dbFiletring = ""

func InitDb() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	APP_DIR = configDir + "/bili-fm"
	if _, err := os.Stat(APP_DIR); os.IsNotExist(err) {
		os.Mkdir(APP_DIR, os.ModePerm)
	}
	dbFiletring = APP_DIR + "/data.db"
	fmt.Println("dbFiletring:", dbFiletring)
}

func GetItem(key string) interface{} {
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
