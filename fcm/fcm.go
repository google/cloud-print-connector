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

type Notification struct {
	Data   [][]interface{}
	Number int
}

type Data struct {
	Notification string `json:"notification"`
	Subtype      string `json:"subtype"`
}

type FcmMessage struct {
	Category    string `json:"category"`
	CollapseKey string `json:"collapse_key"`
	Data        Data   `json:"data"`
	From        string `json:"from"`
	MessageID   string `json:"message_id"`
	TimeToLive  int    `json:"time_to_live"`
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
	iidToken := f.GetToken()
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
		quit <- struct{}{}
		return err
	}
	if resp.StatusCode == 200 {
		reader := bufio.NewReader(resp.Body)
		go func() {
			for {
				line, err := reader.ReadBytes('\n')
				if err == nil || err == io.EOF {
					printerId := GetPrinterID(string(line))
					if printerId != "" {
						pn := notification.PrinterNotification{printerId, notification.PrinterNewJobs}
						go func() {
							fcmNotifications <- pn
						}()
					}
					if err == io.EOF {
						log.Info("DRAIN message received, client reconnecting.")
						dead <- struct{}{}
						break
					}
				} else {
					// stop listening unknown error happened.
					log.Errorf("%v", err)
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
			iidToken := f.GetToken()
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

// Returns cached token and Refresh token if needed.
func (f *FCM) GetToken() string {
	if f.tokenRefreshTime == (time.Time{}) || time.Now().UTC().Sub(f.tokenRefreshTime).Seconds() > f.fcmTTLSecs {
		result, err1 := f.FcmSubscribe(fmt.Sprintf("%s?client=%s&proxy=%s", gcpFcmSubscribePath, f.clientID, f.proxyName))
		if err1 != nil {
			log.Errorf("Unable to subscribe to FCM : %s", err1)
			panic(err1)
		}
		token := result.(map[string]interface{})["token"]
		ttlSeconds, err2 := strconv.ParseFloat(result.(map[string]interface{})["fcmttl"].(string), 64)
		if err2 != nil {
			log.Errorf("Failed to parse FCM ttl  : %s", err2)
			panic(err2)
		}
		f.fcmTTLSecs = ttlSeconds
		log.Info("Updated FCM token.")
		f.cachedToken = token.(string)
		f.tokenRefreshTime = time.Now().UTC()
	}
	return f.cachedToken
}

func GetPrinterID(sLine string) string {
	if strings.HasPrefix(sLine, "[") {
		out := "{" + GetStringInBetween(sLine, "{", "}") + "}"
		var f FcmMessage
		json.Unmarshal([]byte(out), &f)
		if f.Data == (Data{}) {
			return ""
		}
		return f.Data.Notification
	}
	return ""
}

func GetStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.LastIndex(str, end)
	return str[s:e]
}
