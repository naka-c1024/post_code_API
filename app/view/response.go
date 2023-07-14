package view

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func Response(w http.ResponseWriter, data any) {
	// goオブジェクトをjson文字列に変換
	bytes, err := json.Marshal(data)
	if err != nil {
		ErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	// json文字列をレスポンスとして返す
	fmt.Fprint(w, string(bytes))
}

func ErrorResponse(w http.ResponseWriter, err error, statusCode int) {
	http.Error(w, err.Error(), statusCode)
}
