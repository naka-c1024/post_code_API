package model

import "post_code_API/database"

func InsertPostalCode(postalCode string) error {
	// INSERT文の準備、SQLインジェクション対策
	prep, err := database.DB.Prepare("INSERT INTO access_logs(postal_code) VALUES(?)")
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
