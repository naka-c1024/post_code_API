package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type RespGeoAPI struct {
	Response struct {
		Location []struct {
			City       string `json:"city"`
			CityKana   string `json:"city_kana"`
			Town       string `json:"town"`
			TownKana   string `json:"town_kana"`
			X          string `json:"x"`
			Y          string `json:"y"`
			Prefecture string `json:"prefecture"`
			Postal     string `json:"postal"`
		} `json:"location"`
	} `json:"response"`
}

type AddressInfo struct {
	PostalCode       string  `json:"postal_code"`
	HitCount         int     `json:"hit_count"`
	Address          string  `json:"address"`
	TokyoStaDistance float64 `json:"tokyo_sta_distance"`
}

func step1(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "This is step1!\n")
}

func commonPrefix(towns []string) string {
	// 最短の文字列を探す
	shortest := towns[0]
	for _, town := range towns {
		if len(town) < len(shortest) {
			shortest = town
		}
	}

	// runeに変換
	shortestRunes := []rune(shortest)
	commonPrefix := ""

	// 1文字ずつ比較
	for i := 0; i < len(shortestRunes); i++ {
		for _, town := range towns {
			// ひとつでも違う文字があれば共通ではなくなるのでreturn
			if shortestRunes[i] != []rune(town)[i] {
				return commonPrefix
			}
		}
		commonPrefix += string(shortestRunes[i])
	}

	return commonPrefix
}

func insertPostalCode(postalCode string) error {
	// INSERT文の準備、SQLインジェクション対策
	prep, err := DB.Prepare("INSERT INTO access_logs(postal_code) VALUES(?)")
	if err != nil {
		return err
	}
	defer prep.Close()

	// 値をINSERT文に渡す
	if _, err = prep.Exec(postalCode); err != nil {
		return err
	}

	return nil
}

func addressHandler(w http.ResponseWriter, r *http.Request) {
	postalCode := r.FormValue("postal_code")
	if postalCode == "" {
		fmt.Fprintf(w, "Enter postal_code\n")
		return
	}

	url := "https://geoapi.heartrails.com/api/json?method=searchByPostal&postal=" + postalCode
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(w, "http.Get error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// jsonオブジェクトにデコード
	var rga RespGeoAPI
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&rga); err != nil {
		fmt.Fprintf(w, "json.Decode error: %v\n", err)
		return
	}

	var ai AddressInfo

	// リクエストパラメータで与えた郵便番号
	ai.PostalCode = rga.Response.Location[0].Postal

	// 該当する地域の数
	ai.HitCount = len(rga.Response.Location)

	// Geo APIから取得した各住所のうち、共通する部分の住所
	towns := make([]string, ai.HitCount)
	for i, v := range rga.Response.Location {
		towns[i] = v.Town
	}
	ai.Address = rga.Response.Location[0].Prefecture + rga.Response.Location[0].City + commonPrefix(towns)

	// Geo APIから取得した各住所のうち、東京駅から最も離れている地域から東京駅までの距離 [km]
	const tokyoX = 139.7673068 // 東京駅の緯度
	const tokyoY = 35.6809591  // 東京駅の経度
	const earthRadius = 6371.0 // 地球の半径 [km]
	farthestDistance := 0.0
	for i := 0; i < ai.HitCount; i++ {
		x, err := strconv.ParseFloat(rga.Response.Location[i].X, 64)
		if err != nil {
			fmt.Fprintf(w, "strconv.ParseFloat error: %v\n", err)
			return
		}
		y, err := strconv.ParseFloat(rga.Response.Location[i].Y, 64)
		if err != nil {
			fmt.Fprintf(w, "strconv.ParseFloat error: %v\n", err)
			return
		}

		distance := math.Pi * earthRadius / 180 * math.Sqrt(math.Pow((x-tokyoX)*math.Cos(math.Pi*(y+tokyoY)/360), 2)+math.Pow(y-tokyoY, 2))
		if distance > farthestDistance {
			farthestDistance = distance
		}
	}

	// 四捨五入
	const baseNumber = 10 // ⼩数点第⼀位まで
	farthestDistance = math.Round(farthestDistance*baseNumber) / baseNumber
	ai.TokyoStaDistance = farthestDistance

	// jsonオブジェクトをjson文字列に変換
	bytes, err := json.Marshal(ai)
	if err != nil {
		fmt.Fprintf(w, "json.Marshal error: %v\n", err)
		return
	}

	fmt.Fprint(w, string(bytes))

	// データベースにアクセスログを保存
	if err := insertPostalCode(postalCode); err != nil {
		log.Fatal(err)
	}
}

var DB *sql.DB

func init() {
	var err error
	DB, err = connectDB(42)
	if err != nil {
		log.Fatal(err)
	}
	// defer DB.Close()
}

func connectDB(retryCount int) (*sql.DB, error) {
	database_name, ok := os.LookupEnv("MYSQL_DATABASE")
	if !ok {
		return nil, fmt.Errorf("MYSQL_DATABASE is not set")
	}
	user, ok := os.LookupEnv("MYSQL_USER")
	if !ok {
		return nil, fmt.Errorf("MYSQL_USER is not set")
	}
	password, ok := os.LookupEnv("MYSQL_PASSWORD")
	if !ok {
		return nil, fmt.Errorf("MYSQL_PASSWORD is not set")
	}
	host, ok := os.LookupEnv("MYSQL_HOST")
	if !ok {
		return nil, fmt.Errorf("MYSQL_HOST is not set")
	}
	port, ok := os.LookupEnv("MYSQL_PORT")
	if !ok {
		return nil, fmt.Errorf("MYSQL_PORT is not set")
	}

	// [ユーザ名]:[パスワード]@tcp([ホスト名]:[ポート番号])/[データベース名]?charset=[文字コード]&parseTime=true(time.Timeを扱うため)
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=true", user, password, host, port, database_name)
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

// データベースのテーブルの構造体
type DBAccessLogs struct {
	ID         int
	PostalCode string
	CreatedAt  time.Time
}

type AccessLog struct {
	PostalCode   string `json:"postal_code"`
	RequestCount int    `json:"request_count"`
}

type AccessLogInfo struct {
	AccessLogs []AccessLog `json:"access_logs"`
}

func accessLogsHandler(w http.ResponseWriter, r *http.Request) {
	var allAccessLogs AccessLogInfo

	query := `
		SELECT
			postal_code,
			COUNT(*) AS request_count
		FROM
			access_logs
		GROUP BY
			postal_code
		ORDER BY
			request_count DESC
	`

	rows, err := DB.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		currentAccessLog := &AccessLog{}
		if err := rows.Scan(&currentAccessLog.PostalCode, &currentAccessLog.RequestCount); err != nil {
			log.Fatal(err)
		}
		allAccessLogs.AccessLogs = append(allAccessLogs.AccessLogs, *currentAccessLog)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	// jsonオブジェクトをjson文字列に変換
	bytes, err := json.Marshal(allAccessLogs)
	if err != nil {
		fmt.Fprintf(w, "json.Marshal error: %v\n", err)
		return
	}
	fmt.Fprint(w, string(bytes))
}

func main() {
	http.HandleFunc("/", step1)
	http.HandleFunc("/address", addressHandler)
	http.HandleFunc("/address/access_logs", accessLogsHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
