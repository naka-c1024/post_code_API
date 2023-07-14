package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

type errLookupEnv struct {
	err error
}

// エラーが発生した時は内部でそれを保持しつつ、以降の処理は全てパスする
func (e *errLookupEnv) LookupEnv(str string) string {
	if e.err != nil {
		return ""
	}

	env, ok := os.LookupEnv(str)
	if !ok {
		e.err = fmt.Errorf("%s is not set", str)
	}

	return env
}

func (e *errLookupEnv) Err() error {
	return e.err
}

func init() {
	retryCountStr, ok := os.LookupEnv("DB_RETRY_COUNT")
	if !ok {
		log.Fatal("DB_RETRY_COUNT is not set")
	}

	retryCount, err := strconv.Atoi(retryCountStr)
	if err != nil {
		log.Fatal("DB_RETRY_COUNT must be an integer")
	}

	DB, err = connectDB(retryCount)
	if err != nil {
		log.Fatal(err)
	}
}

func connectDB(retryCount int) (*sql.DB, error) {
	// 環境変数を取得
	ele := &errLookupEnv{}
	user := ele.LookupEnv("MYSQL_USER")
	password := ele.LookupEnv("MYSQL_PASSWORD")
	host := ele.LookupEnv("MYSQL_HOST")
	port := ele.LookupEnv("MYSQL_PORT")
	databaseName := ele.LookupEnv("MYSQL_DATABASE")
	// 最後にエラーの有無を確認することで、エラー処理を一カ所にまとめる。
	if ele.Err() != nil {
		return nil, ele.Err()
	}

	// [ユーザ名]:[パスワード]@tcp([ホスト名]:[ポート番号])/[データベース名]?charset=[文字コード]&parseTime=true(time.Timeを扱うため)
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=true", user, password, host, port, databaseName)

	return openDB(dataSourceName, retryCount)
}

func openDB(dataSourceName string, retryCount int) (*sql.DB, error) {
	var err error
	for i := 0; i < retryCount; i++ {
		DB, err = sql.Open("mysql", dataSourceName)
		if err != nil {
			log.Printf("Could not connect to database: %v", err)
			time.Sleep(time.Second * 2)
			continue
		}
		err = DB.Ping()
		if err != nil {
			log.Printf("Could not ping database: %v", err)
			time.Sleep(time.Second * 2)
			continue
		}
		log.Printf("Successfully connected!")
		return DB, nil
	}
	return nil, fmt.Errorf("failed to connect to database after %d retries\n", retryCount)
}
