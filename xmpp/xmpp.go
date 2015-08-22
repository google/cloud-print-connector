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
	PrinterDelete
)

type PrinterNotification struct {
	GCPID string
	Type  PrinterNotificationType
}

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
	go x.keepXMPPAlive()

	return &x, nil
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (x *XMPP) Quit() {
	if x.ix != nil {
		// Signal to KeepXMPPAlive.
		close(x.quit)
		select {
		case <-x.dead:
			// Wait for XMPP to die.
		case <-time.After(3 * time.Second):
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

	// The current access token is the XMPP password.
	ix, err := newInternalXMPP(x.jid, password, x.proxyName, x.server, x.port, x.pingTimeout, x.pingInterval, x.notifications, x.pingIntervalUpdates, x.dead)
	if err != nil {
		return fmt.Errorf("Failed to start XMPP conversation: %s", err)
	}

	x.ix = ix
	return nil
}

// keepXMPPAlive restarts XMPP when it fails.
func (x *XMPP) keepXMPPAlive() {
	for {
		select {
		case <-x.dead:
			glog.Error("XMPP conversation died; restarting")
			if err := x.startXMPP(); err != nil {
				for err != nil {
					glog.Errorf("XMPP restart failed, will try again in 10s: %s", err)
					time.Sleep(10 * time.Second)
					err = x.startXMPP()
				}
				glog.Error("XMPP conversation restarted successfully")
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
