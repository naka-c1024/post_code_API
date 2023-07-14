package main

import (
	"log"
	"net/http"

	"post_code_API/controller"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	http.HandleFunc("/", controller.RootHandler)
	http.HandleFunc("/address", controller.AddressHandler)
	http.HandleFunc("/address/access_logs", controller.AccessLogsHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
