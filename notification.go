package notification

import (
	"fmt"
	"os"

	"github.com/jcodeio/go-common"
)

func SendToUser(userID int, title string, body string, category string, clearFirst bool) {
	var token string

	query := fmt.Sprintf("select token from jcode.device_token where user_id = %d and type = '%s';", userID, os.Getenv("MODE"))

	fmt.Println(query)

	tokenRows, getUserDataErr := common.PG.Query(query)
	defer tokenRows.Close()

	common.CheckError(getUserDataErr)

	for tokenRows.Next() {
		scanErr := tokenRows.Scan(&token)
		common.CheckError(scanErr)

		SendAPNSToDevice(token, title, body, category, clearFirst)
	}
}
