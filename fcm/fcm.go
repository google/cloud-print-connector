package fcm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	backoff backoff
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
		backoff{0, time.Second * 5, time.Minute * 5},
	}
	return &f, nil
}

// Init gets token from GCP and connect to FCM.
func (f *FCM) Init() {
	iidToken := f.GetTokenWithRetry()
	if err := f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit); err != nil {
		for err != nil {
			f.backoff.addError()
			log.Errorf("FCM restart failed, will try again in %4.0f s: %s",
				f.backoff.delay().Seconds(), err)
			time.Sleep(f.backoff.delay())
			err = f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit)
		}
		f.backoff.reset()
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
		// retrying exponentially
		log.Errorf("%v", err)
		return err
	}
	if resp.StatusCode == 200 {
		reader := bufio.NewReader(resp.Body)
		go func() {
			for {
				printerId, err := GetPrinterID(reader)
				if len(printerId) > 0 {
					pn := notification.PrinterNotification{printerId, notification.PrinterNewJobs}
					fcmNotifications <- pn
				}
				if err != nil {
						log.Info("DRAIN message received, client reconnecting.")
						dead <- struct{}{}
						break
					}
			}
		}()
	}
	return nil
}

func GetPrinterID(reader *bufio.Reader) (string, error) {
	raw_input, err := reader.ReadBytes('\n')
	if err == nil {
		// Trim last \n char
		raw_input = raw_input[:len(raw_input) - 1]
		buffer_size, _ := strconv.Atoi(string(raw_input))
		notification_buffer := make([]byte, buffer_size)
		var sofar, sz int
		for err == nil && sofar < buffer_size {
			sz, err = reader.Read(notification_buffer[sofar:])
			sofar += sz
		}

		if sofar == buffer_size {
			var d [][]interface{}
			var f FCMMessage
			json.Unmarshal([]byte(string(notification_buffer)), &d)
			s, _ := json.Marshal(d[0][1])
			json.Unmarshal(s, &f)
			return f[0].Data.Notification, err
		}
	}
	return "", err
}

type backoff struct {
	// The number of consecutive connection errors.
	numErrors uint
	// The minimum amount of time to backoff.
	minBackoff time.Duration
	// The maximum amount of time to backoff.
	maxBackoff time.Duration
}

// Computes the amount of time to delay based on the number of errors.
func (b *backoff) delay() time.Duration {
	if b.numErrors == 0 {
		// Never delay when there are no errors.
		return 0
	}
	curDelay := b.minBackoff
	for i := uint(1); i < b.numErrors; i++ {
		curDelay = curDelay * 2
	}
	if curDelay > b.maxBackoff {
		return b.maxBackoff
	}
	return curDelay
}

// Adds an observed error to inform the backoff delay decision.
func (b *backoff) addError() {
	log.Info("err count")
	b.numErrors++
}

// Resets the backoff back to having no errors.
func (b *backoff) reset() {
	b.numErrors = 0
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
					f.backoff.addError()
					log.Errorf("FCM connection restart failed, will try again in %4.0f s: %s",
						f.backoff.delay().Seconds(), err)
					time.Sleep(f.backoff.delay())
					err = f.ConnectToFcm(f.notifications, iidToken, f.dead, f.quit)
				}
				f.backoff.reset()
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

// GetToken returns cached token and Refresh token if needed.
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