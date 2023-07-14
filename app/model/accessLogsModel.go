package model

import "post_code_API/database"

type AccessLog struct {
	PostalCode   string `json:"postal_code"`
	RequestCount int    `json:"request_count"`
}

type AccessLogInfo struct {
	AccessLogs []AccessLog `json:"access_logs"`
}

func FetchAccessLogs() (AccessLogInfo, error) {
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
	rows, err := database.DB.Query(query)
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
