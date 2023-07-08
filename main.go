package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
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

func address(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprintln(w, string(bytes))
}

func main() {
	http.HandleFunc("/", step1)
	http.HandleFunc("/address", address)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
