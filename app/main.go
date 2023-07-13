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

type GeoAPIResponse struct {
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "This is root!\n")
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

// Geo APIから取得した各住所のうち、東京駅から最も離れている地域から東京駅までの距離 [km]を計算
func calcTokyoStaDistance(addressData AddressInfo, geoData GeoAPIResponse) (float64, error) {
	const tokyoX = 139.7673068 // 東京駅の緯度
	const tokyoY = 35.6809591  // 東京駅の経度
	const earthRadius = 6371.0 // 地球の半径 [km]

	farthestDistance := 0.0
	for i := 0; i < addressData.HitCount; i++ {
		// Geo APIから取得した各住所の緯度・経度をfloat64に変換
		x, err := strconv.ParseFloat(geoData.Response.Location[i].X, 64)
		if err != nil {
			return farthestDistance, err
		}
		y, err := strconv.ParseFloat(geoData.Response.Location[i].Y, 64)
		if err != nil {
			return farthestDistance, err
		}

		// 2点間の距離を計算
		distance := math.Pi * earthRadius / 180 * math.Sqrt(math.Pow((x-tokyoX)*math.Cos(math.Pi*(y+tokyoY)/360), 2)+math.Pow(y-tokyoY, 2))
		if distance > farthestDistance {
			farthestDistance = distance
		}
	}

	// 四捨五入
	const baseNumber = 10 // ⼩数点第⼀位まで
	farthestDistance = math.Round(farthestDistance*baseNumber) / baseNumber

	return farthestDistance, nil
}

func makeAddressData(geoData GeoAPIResponse) (AddressInfo, error) {
	var addressData AddressInfo

	// リクエストパラメータで与えた郵便番号
	addressData.PostalCode = geoData.Response.Location[0].Postal

	// 該当する地域の数
	addressData.HitCount = len(geoData.Response.Location)

	// Geo APIから取得した各住所のうち、共通する部分の住所
	towns := make([]string, addressData.HitCount)
	for i, v := range geoData.Response.Location {
		towns[i] = v.Town
	}
	addressData.Address = geoData.Response.Location[0].Prefecture + geoData.Response.Location[0].City + commonPrefix(towns)

	// Geo APIから取得した各住所のうち、東京駅から最も離れている地域から東京駅までの距離 [km]を計算
	tokyoStaDistance, err := calcTokyoStaDistance(addressData, geoData)
	if err != nil {
		return addressData, err
	}
	addressData.TokyoStaDistance = tokyoStaDistance

	return addressData, nil
}

func fetchGeoData(postalCode string) (GeoAPIResponse, error, int) {
	var geoData GeoAPIResponse

	// Geo APIにリクエスト
	url := "https://geoapi.heartrails.com/api/json?method=searchByPostal&postal=" + postalCode
	resp, err := http.Get(url)
	if err != nil {
		return geoData, err, http.StatusBadRequest
	}
	defer resp.Body.Close()

	// goオブジェクトにデコード
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&geoData); err != nil {
		return geoData, err, http.StatusInternalServerError
	}

	return geoData, nil, http.StatusOK
}

func addressHandler(w http.ResponseWriter, r *http.Request) {
	postalCode := r.FormValue("postal_code")
	if postalCode == "" {
		fmt.Fprintf(w, "Enter postal_code\n")
		return
	}

	// Geo APIからデータを取得し、goオブジェクトにして格納
	geoData, err, statusCode := fetchGeoData(postalCode)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	// レスポンス用のデータを作成
	addressData, err := makeAddressData(geoData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// goオブジェクトをjson文字列に変換
	bytes, err := json.Marshal(addressData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// json文字列をレスポンスとして返す
	fmt.Fprint(w, string(bytes))

	// データベースにアクセスログを保存
	if err := insertPostalCode(postalCode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var DB *sql.DB

func init() {
	var err error
	DB, err = connectDB(42)
	if err != nil {
		log.Fatal(err)
	}
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

type AccessLog struct {
	PostalCode   string `json:"postal_code"`
	RequestCount int    `json:"request_count"`
}

type AccessLogInfo struct {
	AccessLogs []AccessLog `json:"access_logs"`
}

func fetchAccessLogs() (AccessLogInfo, error) {
	var allAccessLogs AccessLogInfo

	const query = `
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

	// SELECT文の実行
	rows, err := DB.Query(query)
	if err != nil {
		return allAccessLogs, err
	}
	defer rows.Close()

	// データベースから取得した値を格納
	for rows.Next() {
		currentAccessLog := &AccessLog{}
		if err := rows.Scan(&currentAccessLog.PostalCode, &currentAccessLog.RequestCount); err != nil {
			return allAccessLogs, err
		}
		allAccessLogs.AccessLogs = append(allAccessLogs.AccessLogs, *currentAccessLog)
	}

	err = rows.Err()
	if err != nil {
		return allAccessLogs, err
	}

	return allAccessLogs, nil
}

func accessLogsHandler(w http.ResponseWriter, r *http.Request) {
	// データベースからアクセスログを取得
	allAccessLogs, err := fetchAccessLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// goオブジェクトをjson文字列に変換
	bytes, err := json.Marshal(allAccessLogs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// json文字列をレスポンスとして返す
	fmt.Fprint(w, string(bytes))
}

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/address", addressHandler)
	http.HandleFunc("/address/access_logs", accessLogsHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
