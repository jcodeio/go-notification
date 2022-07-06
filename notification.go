package notification

import (
	"fmt"
	"os"

	"jcode.io/go/db"
)

func SendToUser(userID int, title string, body string, category string, clearFirst bool) {
	var token string

	query := fmt.Sprintf("select token from public.device_token where user_id = %d and type = '%s';", userID, os.Getenv("MODE"))

	fmt.Println(query)

	tokenRows, getUserDataErr := db.PG.Query(query)
	defer tokenRows.Close()

	if getUserDataErr != nil {
		fmt.Println(getUserDataErr)
		return
	}
	for tokenRows.Next() {
		if scanErr := tokenRows.Scan(&token); scanErr != nil {
			fmt.Println(scanErr)
			continue
		}
		fmt.Println(token)
		SendAPNSToDevice(token, title, body, category, clearFirst)
	}
}
