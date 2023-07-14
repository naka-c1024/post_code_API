package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"post_code_API/model"
	"post_code_API/view"
	"strconv"
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

func commonPrefix(towns []string) (string, error) {
	if len(towns) == 0 {
		return "", fmt.Errorf("towns is empty")
	}

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
				return commonPrefix, nil
			}
		}
		commonPrefix += string(shortestRunes[i])
	}

	return commonPrefix, nil
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

	// 7桁の数字だったが、存在しない郵便番号の場合
	if len(geoData.Response.Location) == 0 {
		return addressData, fmt.Errorf("postal_code is invalid")
	}

	// リクエストパラメータで与えた郵便番号
	addressData.PostalCode = geoData.Response.Location[0].Postal

	// 該当する地域の数
	addressData.HitCount = len(geoData.Response.Location)

	// Geo APIから取得した各住所のうち、共通する部分の住所
	towns := make([]string, addressData.HitCount)
	for i, v := range geoData.Response.Location {
		towns[i] = v.Town
	}
	commonTown, err := commonPrefix(towns)
	if err != nil {
		return addressData, err
	}
	addressData.Address = geoData.Response.Location[0].Prefecture + geoData.Response.Location[0].City + commonTown

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
	if err != nil || resp.StatusCode != http.StatusOK {
		return geoData, err, resp.StatusCode
	}
	defer resp.Body.Close()

	// goオブジェクトにデコード
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&geoData); err != nil {
		return geoData, err, http.StatusInternalServerError
	}

	return geoData, nil, http.StatusOK
}

func getPostalCode(r *http.Request) (string, error) {
	postalCode := r.FormValue("postal_code")

	if len(postalCode) != 7 {
		return "", fmt.Errorf("postal_code is invalid")
	} else if _, err := strconv.Atoi(postalCode); err != nil {
		return "", fmt.Errorf("postal_code is invalid")
	}
	return postalCode, nil
}

func AddressHandler(w http.ResponseWriter, r *http.Request) {
	// リクエストパラメータから郵便番号を取得
	postalCode, err := getPostalCode(r)
	if err != nil {
		view.ErrorResponse(w, err, http.StatusBadRequest)
		return
	}

	// Geo APIからデータを取得し、goオブジェクトにして格納
	geoData, err, statusCode := fetchGeoData(postalCode)
	if err != nil || statusCode != http.StatusOK {
		view.ErrorResponse(w, err, statusCode)
		return
	}

	// レスポンス用のデータを作成
	addressData, err := makeAddressData(geoData)
	if err != nil {
		view.ErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	// データベースにアクセスログを保存
	if err := model.InsertPostalCode(postalCode); err != nil {
		view.ErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	view.Response(w, addressData)
}
