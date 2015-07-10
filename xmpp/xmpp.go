/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package xmpp

import (
	"fmt"
	"time"

	"github.com/golang/glog"
)

type PrinterNotificationType uint8

const (
	PrinterNewJobs PrinterNotificationType = iota
	PrinterUpdate
	PrinterDelete
)

type PrinterNotification struct {
	GCPID string
	Type  PrinterNotificationType
}

const (
	// XMPP connections fail. Attempt to reconnect a few times before giving up.
	restartXMPPMaxRetries = 4
)

type XMPP struct {
	jid            string
	proxyName      string
	server         string
	port           uint16
	pingTimeout    time.Duration
	pingInterval   time.Duration
	getAccessToken func() (string, error)

	notifications       chan PrinterNotification
	pingIntervalUpdates chan time.Duration
	dead                chan struct{}

	quit chan struct{}

	ix *internalXMPP
}

func NewXMPP(jid, proxyName, server string, port uint16, pingTimeout, pingInterval time.Duration, getAccessToken func() (string, error)) (*XMPP, error) {
	x := XMPP{
		jid:                 jid,
		proxyName:           proxyName,
		server:              server,
		port:                port,
		pingTimeout:         pingTimeout,
		pingInterval:        pingInterval,
		getAccessToken:      getAccessToken,
		notifications:       make(chan PrinterNotification, 10),
		pingIntervalUpdates: make(chan time.Duration, 10),
		dead:                make(chan struct{}),
		quit:                make(chan struct{}),
	}

	err := x.startXMPP()
	if err != nil {
		return nil, err
	}

	return &x, nil
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (x *XMPP) Quit() {
	if x.ix != nil {
		// Signal to KeepXMPPAlive.
		x.quit <- struct{}{}
		select {
		case <-x.dead:
			// Wait for XMPP to die.
		case <-time.After(5 * time.Second):
			// But not too long.
			glog.Error("XMPP taking a while to close, so giving up")
		}
	}
}

// startXMPP tries to start an XMPP conversation.
// Tries multiple times before returning an error.
func (x *XMPP) startXMPP() error {
	if x.ix != nil {
		go x.ix.Quit()
	}

	password, err := x.getAccessToken()
	if err != nil {
		return fmt.Errorf("While starting XMPP, failed to get access token (password): %s", err)
	}

	for i := 0; i < restartXMPPMaxRetries; i++ {
		// The current access token is the XMPP password.
		if err == nil {
			var ix *internalXMPP
			ix, err := newInternalXMPP(x.jid, password, x.proxyName, x.server, x.port, x.pingTimeout, x.pingInterval, x.notifications, x.pingIntervalUpdates, x.dead)

			if err == nil {
				// Success!
				x.ix = ix
				// Don't give up.
				go x.keepXMPPAlive()
				return nil
			}
		}

		// Sleep for 1, 2, 4, 8 seconds.
		time.Sleep(time.Duration((i+1)*2) * time.Second)
	}

	return fmt.Errorf("Failed to start XMPP conversation: %s", err)
}

// keepXMPPAlive restarts XMPP when it fails.
func (x *XMPP) keepXMPPAlive() {
	for {
		select {
		case <-x.dead:
			glog.Error("XMPP conversation died; restarting")
			if err := x.startXMPP(); err != nil {
				glog.Fatalf("Failed to keep XMPP conversation alive: %s", err)
			}
		case <-x.quit:
			// Close XMPP.
			x.ix.Quit()
			return
		}
	}
}

// Notifications returns a channel on which PrinterNotifications arrive.
func (x *XMPP) Notifications() <-chan PrinterNotification {
	return x.notifications
}

// SetPingInterval sets the XMPP ping interval. Should be the min of all
// printers' ping intervals.
func (x *XMPP) SetPingInterval(interval time.Duration) {
	x.pingIntervalUpdates <- interval
	glog.Infof("Connector XMPP ping interval changed to %s", interval.String())
}
