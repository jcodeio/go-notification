package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jcodeio/go-common"
)

var lastJWTUpdate time.Time
var auth string
var HostUrl string

var apnsTopic string
var apnsSound string

func init() {
	// get jwt
	lastJWTUpdate = time.Now()
	updateJWT()

	// get values from env for headers
	apnsTopic = os.Getenv("APNS_TOPIC")
	if apnsTopic == "" {
		fmt.Printf("APNS_TOPIC Required in .env file")
		os.Exit(1)
	}
	apnsSound = os.Getenv("APNS_SOUND")
	if apnsSound == "" {
		fmt.Printf("APNS_SOUND Required in .env file")
		os.Exit(1)
	}
}

func updateJWT() {
	// get values from env file
	teamID := os.Getenv("APNS_TEAMID")
	if teamID == "" {
		fmt.Printf("APNS_TEAMID Required in .env file")
		os.Exit(1)
	}
	p8Path := os.Getenv("APNS_P8_PATH")
	if p8Path == "" {
		fmt.Printf("APNS_P8_PATH Required in .env file")
		os.Exit(1)
	}
	if !common.FileExists(p8Path) {
		fmt.Printf("apns p8 file not found")
		os.Exit(1)
	}
	authKey := os.Getenv("APNS_AUTHKEY")
	if authKey == "" {
		fmt.Printf("APNS_AUTHKEY Required in .env file")
		os.Exit(1)
	}

	// generate a jwt - parameters are team id, p8 path, and auth key
	out, jwtErr := exec.Command("/bin/bash", fmt.Sprintf("./jcode/notification/apns/getjwt.sh %s %s %s", teamID, p8Path, authKey)).Output()

	if jwtErr != nil {
		fmt.Printf("failed to generate jwt, error: %s", jwtErr)
	}

	// token comes from getjwt.sh in this directory
	auth = strings.TrimSuffix("Bearer "+string(out), "\n")
	lastJWTUpdate = time.Now()
}

type notif struct {
	Aps aps `json:"aps"`
}

type aps struct {
	Alert    alert  `json:"alert"`
	Category string `json:"category"`
	Badge    int    `json:"badge"`
	Sound    string `json:"sound"`
}

type alert struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body"`
}

type FailedResponse struct {
	Reason string `json:"reason"`
}

var client = &http.Client{}

func SendAPNSToDevice(token string, title string, body string, category string, clearFirst bool) bool {
	// check if jwt is older than an hour, if so refresh
	if lastJWTUpdate.Add(time.Minute*55).Unix() < time.Now().Unix() {
		updateJWT()
	}

	// create notification payload
	data := notif{
		Aps: aps{
			Alert: alert{
				Title: title,
				Body:  body,
			},
			Category: category,
			Badge:    1,
			Sound:    apnsSound,
		},
	}

	dataBytes, bodyErr := json.Marshal(data)
	if bodyErr != nil {
		fmt.Println(bodyErr)
	}

	if clearFirst {
		// if the clear fails, no reason to try the correct one
		if !clearPastNotifications(token) {
			return false
		}
	}

	// create notification post request
	req, createPostErr := http.NewRequest("POST", getNotificationURL(token), bytes.NewReader(dataBytes))
	if createPostErr != nil {
		fmt.Println(createPostErr)
		return false
	}

	// apns headers
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-expiration", fmt.Sprint(time.Now().Add(time.Hour*24).Unix()))
	req.Header.Set("apns-topic", apnsTopic)
	req.Header.Set("Authorization", auth)

	return sendAPNSRequest(req, token)
}

func getNotificationURL(token string) string {
	path := "/3/device/" + token
	return HostUrl + path
}

func clearPastNotifications(token string) bool {
	// apns set badge to 0 to clear
	data := notif{
		Aps: aps{
			Badge: 0,
		},
	}
	dataBytes, bodyErr := json.Marshal(data)
	if bodyErr != nil {
		fmt.Println(bodyErr)
	}
	req, createPostErr := http.NewRequest("POST", getNotificationURL(token), bytes.NewReader(dataBytes))
	if createPostErr != nil {
		fmt.Println(createPostErr)
		return false
	}
	// apns headers
	req.Header.Set("apns-topic", apnsTopic)
	req.Header.Set("Authorization", auth)

	return sendAPNSRequest(req, token)
}

func sendAPNSRequest(req *http.Request, token string) bool {
	response, apnsPostErr := client.Do(req)

	if apnsPostErr != nil {
		fmt.Printf("failed to send notification, error: %s", apnsPostErr)
		return false
	}

	defer response.Body.Close()

	fmt.Println(response)

	if response.StatusCode != http.StatusOK {
		var failedResp FailedResponse
		body, readErr := ioutil.ReadAll(response.Body)
		if readErr != nil {
			fmt.Printf("failed to read apns response: %s", readErr)
		}
		parseErr := json.Unmarshal(body, &failedResp)
		if parseErr != nil {
			fmt.Printf("failed to read apns response: %s", parseErr)
		}
		fmt.Printf("failed %s %s\n", failedResp.Reason, token)
		if failedResp.Reason == "Unregistered" || failedResp.Reason == "BadDeviceToken" {
			// get rid of bad tokens
			deleteToken(token)
		}
		return false
	} else {
		return true
	}
}
func deleteToken(token string) {
	query := fmt.Sprintf(`delete from jcode.device_token where token = '%s';`, token)
	_, delErr := common.PG.Exec(query)
	if delErr != nil {
		fmt.Println(delErr)
		return
	}
}
