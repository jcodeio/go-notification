package notification

import (
	"fmt"
	"time"

	"github.com/jcodeio/go-common"
)

func StartPoll() {
	for {
		getPendingNotifications()
		// wait 8 seconds to check again
		time.Sleep(8 * time.Second)
	}
}

func getPendingNotifications() {
	var now = time.Now().UTC().Unix()
	// get notifications we passed
	// only get users in respective release
	query := fmt.Sprintf(`select distinct pn.user_id, dt.token, pn.notification_id as notification_id, pn.header, pn.message, pn.category, pn.type from public.pending_notification pn 
	join public.device_token dt on dt.user_id = pn.user_id 
	join public.user us on us.user_id = pn.user_id 
	join (select max(notification_id) as notification_id, user_id from public.pending_notification pn2 where pn2.sched_date <= to_timestamp(%d) 
	group by user_id) pn2 on pn.notification_id = pn2.notification_id 
	where pn.sched_date <= to_timestamp(%d) and dt.type = '%s' and us.release_mode = '%s';`, now, now, common.Mode, common.Mode)

	pendingRows, pendingNotifErr := common.PG.Query(query)
	common.CheckError(pendingNotifErr)
	defer pendingRows.Close()

	var userID int
	var token string
	var notificationID int
	var header string
	var message string
	var category string
	var notifType string

	var deleteAndLogQuery string

	for pendingRows.Next() {
		if scanErr := pendingRows.Scan(&userID, &token, &notificationID, &message, &notifType, &header, &category); scanErr != nil {
			fmt.Println(scanErr)
			return
		}

		fmt.Println(notifType)

		// set pending to active
		if notifType == "session" {
			_, err := common.PG.Exec(fmt.Sprintf(`update public.user set pending_answer = true where user_id = %d;`, userID))
			common.CheckError(err)
		}

		_ = SendAPNSToDevice(token, header, message, category, true)

		deleteAndLogQuery += notificationLogAndDeleteQuery(userID, notificationID)
	}

	// delete sent notifications
	_, delAndLogErr := common.PG.Exec(deleteAndLogQuery)
	common.CheckError(delAndLogErr)

}
func notificationLogAndDeleteQuery(userID int, notificationID int) string {
	// move to logs and delete from pending
	// todo history
	query := fmt.Sprintf(`delete from public.pending_notification where notification_id <= %d and user_id = %d;`, notificationID, userID)
	return query
}
