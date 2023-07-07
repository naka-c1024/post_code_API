package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func step1(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "This is step1!\n")
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
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(w, "io.ReadAll error: %v\n", err)
	}

	fmt.Fprintf(w, string(body)) // 取得した情報は[]byteなのでstringに型変換
}

func main() {
	http.HandleFunc("/", step1)
	http.HandleFunc("/address", address)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
