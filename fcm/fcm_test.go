package fcm_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

import (
	"github.com/google/cloud-print-connector/fcm"
	"github.com/google/cloud-print-connector/notification"
)

func TestFCM_ReceiveNotification(t *testing.T) {

	// test FCM server
	handler := func(w http.ResponseWriter, r *http.Request) {
		fcmTokenResponse := `[[4,[{"from":"xyz","category":"js","collapse_key":"xyz","data":{"notification":"printerId","subtype":"xyz"},"message_id":"xyz","time_to_live":60}]]]10`
		fmt.Fprint(w, fcmTokenResponse)
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	var f *fcm.FCM
	notifications := make(chan notification.PrinterNotification, 5)

	// sample notification
	var printerNotification map[string]interface{}
	printerNotificationStr := `{"fcmttl":"2419200","request":{"params":{"client":["xyz"],"proxy":["xyz"]},"time":"0","user":"xyz","users":["xyz"]},"success":true,"token":"token","xsrf_token":"xyz"}`
	json.Unmarshal([]byte(printerNotificationStr), &printerNotification)

	f, err := fcm.NewFCM("clientid", "", ts.URL, func(input string) (interface{}, error) { return printerNotification, nil }, notifications)
	defer f.Quit()
	if err != nil {
		t.Fatal(err)
	}

	// This method is stubbed.
	result, err := f.FcmSubscribe("SubscribeUrl")
	if err != nil {
		t.Fatal(err)
	}

	token := result.(map[string]interface{})["token"].(string)
	dead := make(chan struct{})
	quit := make(chan struct{})

	// bind to FCM to receive notifications
	f.ConnectToFcm(notifications, token, dead, quit)
	go func() {
		time.Sleep(1 * time.Second)
		// time out
		notifications <- notification.PrinterNotification{"dummy", notification.PrinterNewJobs}
	}()
	message := <-notifications

	// verify if right message received.
	if message.GCPID != "printerId" {
		t.Fatal("Did not receive right printer notification")
	}

}
