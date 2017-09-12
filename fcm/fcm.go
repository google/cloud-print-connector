package fcm

import (
	"net/http"
)
import (
	"bufio"
	"github.com/google/cloud-print-connector/log"
	"strings"
	"encoding/json"
	"fmt"
	"time"
	"github.com/google/cloud-print-connector/gcp"
	"github.com/google/cloud-print-connector/notification"
	"strconv"
)

const (
	gcpFcmServerURL   = "https://fcm-stream.googleapis.com/fcm/"
	gcpFcmBindPath    = "connect/bind"
	gcpFcmSubscribePath    = "fcm/subscribe"
)

type FCM struct {
	fcmServer        string
	bindPath         string
	cachedToken      string
	tokenRefreshTime time.Time
	clientId         string
	proxyName        string
	fcmTtlSecs       float64
	g                *gcp.GoogleCloudPrint

	notifications chan<- notification.PrinterNotification
	dead          chan struct{}

	quit chan struct{}
}

type Notification struct {
	Data                  [][]interface{}
	Number                int
}

type Data struct {
	Notification string `json:"notification"`
	Subtype      string `json:"subtype"`
}

type FcmMessage struct {
	Category    string `json:"category"`
	CollapseKey string `json:"collapse_key"`
	Data        Data `json:"data"`
	From       string `json:"from"`
	MessageID  string `json:"message_id"`
	TimeToLive int    `json:"time_to_live"`
}

func NewFCM(g *gcp.GoogleCloudPrint, notifications chan<- notification.PrinterNotification, clientId string, proxyName string) (*FCM, error) {
	f := FCM{
		gcpFcmServerURL,
		gcpFcmBindPath,
		"",
		time.Time{},
		clientId,
		proxyName,
		0,
		g,
		notifications,
		make(chan struct{}),
		make(chan struct{}),
	}
	return &f, nil
}

func (f *FCM) Init()  {
	if err := f.ConnectToFcm(f.notifications, f.dead, f.quit); err != nil {
		for err != nil {
			log.Errorf("FCM restart failed, will try again in 10s: %s", err)
			time.Sleep(10 * time.Second)
			err = f.ConnectToFcm(f.notifications, f.dead, f.quit)
		}
		log.Error("FCM conversation restarted successfully")
	}

	go f.KeepFcmAlive()
}

// Quit terminates the FCM conversation so that new jobs stop arriving.
func (f *FCM) Quit() {
	// Signal to KeepFCMAlive.
	close(f.quit)
}

func (f *FCM) ConnectToFcm(fcmNotifications chan<- notification.PrinterNotification, dead chan<- struct{}, quit chan<- struct{}) (error){
	log.Debugf("Connecting to %s%s?token=%s", f.fcmServer, f.bindPath, f.GetToken())
	resp, err := http.Get(fmt.Sprintf("%s%s?token=%s", f.fcmServer, f.bindPath, f.GetToken()))
	if err != nil {
		// failed for ever no need to retry
		quit <- struct {}{}
		return err
	}
	if resp.StatusCode == 200 {
		reader := bufio.NewReader(resp.Body)
		go func (){
		for {
			line, err := reader.ReadBytes('\n')
			sLine := string(line)
			if err != nil {
				// just drain so reconnect
				log.Info("FCM client reconnected.")
				dead <- struct{}{}
				break
			}
			printerId := GetPrinterID(sLine)
			if printerId != "" {
				pn := notification.PrinterNotification{printerId, notification.PrinterNewJobs}
				go func() {
					fcmNotifications <- pn
				}()
			}
		}

	}()
	}
	return nil
}

// keepFCMAlive restarts FCM when it fails.
func (f *FCM) KeepFcmAlive() {
	for {
		select {
		case <-f.dead:
			log.Error("FCM conversation died; restarting")
			if err := f.ConnectToFcm(f.notifications, f.dead, f.quit); err != nil {
				for err != nil {
					log.Errorf("FCM connection restart failed, will try again in 10s: %s", err)
					time.Sleep(10 * time.Second)
					err = f.ConnectToFcm(f.notifications, f.dead, f.quit)
				}
				log.Error("FCM conversation restarted successfully")
			}

		case <-f.quit:
			// quitting keeping alive
			return
		}
	}
}

func (f *FCM) GetToken() (string){
	if f.tokenRefreshTime == (time.Time{}) || time.Now().UTC().Sub(f.tokenRefreshTime).Seconds() > f.fcmTtlSecs {
		result, err1 := f.g.FcmSubscribe(fmt.Sprintf("%s?client=%s&proxy=%s", gcpFcmSubscribePath, f.clientId, f.proxyName))
		if err1 != nil {
			log.Errorf("Unable to subscribe to FCM : %s", err1)
			fmt.Printf("Unable to subscribe to FCM : %s\n", err1)
			panic(err1)
		}
		token := result.(map[string]interface{})["token"]
		ttlSeconds , err2 := strconv.ParseFloat(result.(map[string]interface{})["fcmttl"].(string), 64)
		if err2 != nil {
			log.Errorf("Failed to parse FCM ttl  : %s", err2)
			fmt.Printf("Failed to parse FCM ttl  : %s\n", err2)
			panic(err2)
		}
		f.fcmTtlSecs = ttlSeconds
		log.Info("Updated FCM token.")
		fmt.Println("Updated FCM token.")
		f.cachedToken = token.(string)
		f.tokenRefreshTime = time.Now().UTC()
	}
	return f.cachedToken
}

func GetPrinterID(sLine string) (string){
	if strings.HasPrefix(sLine, "[") {
		out := "{" + GetStringInBetween(sLine, "{", "}") + "}"
		var f FcmMessage
		json.Unmarshal([]byte(out), &f)
		if f.Data == (Data {}){
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
