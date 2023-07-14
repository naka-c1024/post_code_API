package controller

import (
	"net/http"

	"post_code_API/model"
	"post_code_API/view"
)

func AccessLogsHandler(w http.ResponseWriter, r *http.Request) {
	// データベースからアクセスログを取得
	allAccessLogs, err := model.FetchAccessLogs()
	if err != nil {
		view.ErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	view.Response(w, allAccessLogs)
}
