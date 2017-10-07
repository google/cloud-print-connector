package fcm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

import (
	"github.com/google/cloud-print-connector/log"
	"github.com/google/cloud-print-connector/notification"
)

const (
	gcpFcmSubscribePath = "fcm/subscribe"
)

type FCM struct {
	fcmServerBindURL string
	cachedToken      string
	tokenRefreshTime time.Time
	clientID         string
	proxyName        string
	fcmTTLSecs       float64
	FcmSubscribe     func(string) (interface{}, error)

	notifications chan<- notification.PrinterNotification
	dead          chan struct{}

	quit chan struct{}
}

type FCMMessage []struct {
	From        string `json:"from"`
	Category    string `json:"category"`
	CollapseKey string `json:"collapse_key"`
	Data struct {
		Notification string `json:"notification"`
		Subtype      string `json:"subtype"`
	} `json:"data"`
	MessageID  string `json:"message_id"`
	TimeToLive int    `json:"time_to_live"`
}

func NewFCM(clientID string, proxyName string, fcmServerBindURL string, FcmSubscribe func(string) (interface{}, error), notifications chan<- notification.PrinterNotification) (*FCM, error) {
	f := FCM{
		fcmServerBindURL,
		"",
		time.Time{},
		clientID,
		proxyName,
		0,
		FcmSubscribe,
		notifications,
		make(chan struct{}),
		make(chan struct{}),
	}
	return &f, nil
}

//  get token from GCP and connect to FCM.
func (f *FCM) Init() {
	iidToken := f.GetTokenWithRetry()
	if err := f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit); err != nil {
		for err != nil {
			log.Errorf("FCM restart failed, will try again in 10s: %s", err)
			time.Sleep(10 * time.Second)
			err = f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit)
		}
		log.Info("FCM conversation restarted successfully")
	}

	go f.KeepFcmAlive()
}

// Quit terminates the FCM conversation so that new jobs stop arriving.
func (f *FCM) Quit() {
	// Signal to KeepFCMAlive.
	close(f.quit)
}

// Fcm notification listener
func (f *FCM) ConnectToFcm(fcmNotifications chan<- notification.PrinterNotification, iidToken string, dead chan<- struct{}, quit chan<- struct{}) error {
	log.Debugf("Connecting to %s?token=%s", f.fcmServerBindURL, iidToken)
	resp, err := http.Get(fmt.Sprintf("%s?token=%s", f.fcmServerBindURL, iidToken))
	if err != nil {
		// failed for ever no need to retry
		log.Errorf("%v", err)
		quit <- struct{}{}
		return err
	}
	if resp.StatusCode == 200 {
		reader := bufio.NewReader(resp.Body)
		go func() {
			for {
				raw_input, err1 := reader.ReadBytes('\n')
				input_chunks := strings.SplitN(string(raw_input), "\n", 2)
				notification_size := strings.TrimSpace(input_chunks[0])
				notification_data1 := ""
				if len(input_chunks) > 1 {
					notification_data1 = strings.TrimSpace(input_chunks[1])
				}
				size, _ := strconv.Atoi(notification_size)
				buffer_size := size - len(notification_data1)

				notification_data2 := ""
				var err2 error
				for 0 != buffer_size {
					// part of notification
					notification_data2_buffer := make([]byte, buffer_size)
					n, err2 := reader.Read(notification_data2_buffer)
					notification_data2 += string(notification_data2_buffer)
					buffer_size -= n

					if err2 != nil {
						break
					}
				}
				// process EOF signal after processing notification.
				if err1 == io.EOF || err2 == io.EOF || (err2 == nil && err1 == nil) {
					notification_string := notification_data1 + notification_data2
					if len(notification_string) > 0 {
						printerId := GetPrinterID(notification_string)
						if printerId != "" {
							pn := notification.PrinterNotification{printerId, notification.PrinterNewJobs}
							fcmNotifications <- pn
						}
					}
					if err2 == io.EOF || err1 == io.EOF {
						log.Info("DRAIN message received, client reconnecting.")
						dead <- struct{}{}
						break
					}
				} else {
					// stop listening unknown error happened.
					log.Errorf("Unexpected error happened on FCM listener: %v, %v", err1, err2)
					quit <- struct{}{}
					break
				}
			}
		}()
	}
	return nil
}

// Restart FCM connection when lost.
func (f *FCM) KeepFcmAlive() {
	for {
		select {
		case <-f.dead:
			iidToken := f.GetTokenWithRetry()
			log.Error("FCM conversation died; restarting")
			if err := f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit); err != nil {
				for err != nil {
					log.Errorf("FCM connection restart failed, will try again in 10s: %s", err)
					time.Sleep(10 * time.Second)
					err = f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit)
				}
				log.Info("FCM conversation restarted successfully")
			}

		case <-f.quit:
			log.Info("Fcm client Quitting ...")
			// quitting keeping alive
			return
		}
	}
}

func (f *FCM) GetTokenWithRetry() string {
	retryCount := 3
	iidToken, err := f.GetToken()
	for err != nil && retryCount < 3 {
		retryCount -= 1
		log.Errorf("unable to get FCM token from GCP server, will try again in 10s: %s", err)
		time.Sleep(10 * time.Second)
		iidToken, err = f.GetToken()
	}
	if err != nil {
		log.Errorf("unable to get FCM token from GCP server.")
		panic(err)
	}
	return iidToken
}

// Returns cached token and Refresh token if needed.
func (f *FCM) GetToken() (string, error) {
	if f.tokenRefreshTime == (time.Time{}) || time.Now().UTC().Sub(f.tokenRefreshTime).Seconds() > f.fcmTTLSecs {
		result, err := f.FcmSubscribe(fmt.Sprintf("%s?client=%s&proxy=%s", gcpFcmSubscribePath, f.clientID, f.proxyName))
		if err != nil {
			log.Errorf("Unable to subscribe to FCM : %s", err)
			return "", err
		}
		token := result.(map[string]interface{})["token"]
		ttlSeconds, err := strconv.ParseFloat(result.(map[string]interface{})["fcmttl"].(string), 64)
		if err != nil {
			log.Errorf("Failed to parse FCM ttl  : %s", err)
			return "", err
		}
		f.fcmTTLSecs = ttlSeconds
		log.Info("Updated FCM token.")
		f.cachedToken = token.(string)
		f.tokenRefreshTime = time.Now().UTC()
	}
	return f.cachedToken, nil
}

func GetPrinterID(sLine string) string {
	var d [][]interface{}
	var f FCMMessage
	json.Unmarshal([]byte(sLine), &d)
	s, _ := json.Marshal(d[0][1])
	json.Unmarshal(s, &f)
	return f[0].Data.Notification
}
