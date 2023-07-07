package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

func commonPrefix(slices []string) string {
	// 最短の文字列を探す
	shortest := slices[0]
	for _, slice := range slices {
		if len(slice) < len(shortest) {
			shortest = slice
		}
	}

	// runeに変換
	shortestRunes := []rune(shortest)
	commonPrefix := ""

	for i := 0; i < len(shortestRunes); i++ {
		for _, slice := range slices {
			// 共通部分を探す
			if shortestRunes[i] != []rune(slice)[i] {
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

	// jsonオブジェクトをjson文字列に変換
	// bytes, err := json.Marshal(rga)
	// if err != nil {
	// 	fmt.Fprintf(w, "json.Marshal error: %v\n", err)
	// 	return
	// }
	// fmt.Fprintln(w, string(bytes))

	var ai AddressInfo
	ai.PostalCode = rga.Response.Location[0].Postal
	ai.HitCount = len(rga.Response.Location)

	towns := make([]string, ai.HitCount)
	for i, v := range rga.Response.Location {
		towns[i] = v.Town
	}
	ai.Address = rga.Response.Location[0].Prefecture + rga.Response.Location[0].City + commonPrefix(towns)

	ai.TokyoStaDistance = 0.0

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
