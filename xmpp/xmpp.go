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

	"github.com/google/cloud-print-connector/log"
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

	notifications chan<- PrinterNotification
	dead          chan struct{}

	quit chan struct{}

	ix *internalXMPP
}

func NewXMPP(jid, proxyName, server string, port uint16, pingTimeout, pingInterval time.Duration, getAccessToken func() (string, error), notifications chan<- PrinterNotification) (*XMPP, error) {
	x := XMPP{
		jid:            jid,
		proxyName:      proxyName,
		server:         server,
		port:           port,
		pingTimeout:    pingTimeout,
		pingInterval:   pingInterval,
		getAccessToken: getAccessToken,
		notifications:  notifications,
		dead:           make(chan struct{}),
		quit:           make(chan struct{}),
	}

	if err := x.startXMPP(); err != nil {
		for err != nil {
			log.Errorf("XMPP start failed, will try again in 10s: %s", err)
			time.Sleep(10 * time.Second)
			err = x.startXMPP()
		}
	}
	go x.keepXMPPAlive()

	return &x, nil
}

// Quit terminates the XMPP conversation so that new jobs stop arriving.
func (x *XMPP) Quit() {
	// Signal to KeepXMPPAlive.
	close(x.quit)
	select {
	case <-x.dead:
		// Wait for XMPP to die.
	case <-time.After(3 * time.Second):
		// But not too long.
		log.Error("XMPP taking a while to close, so giving up")
	}
}

// startXMPP tries to start an XMPP conversation.
func (x *XMPP) startXMPP() error {
	if x.ix != nil {
		go x.ix.Quit()
		x.ix = nil
	}

	password, err := x.getAccessToken()
	if err != nil {
		return fmt.Errorf("While starting XMPP, failed to get access token (password): %s", err)
	}

	// The current access token is the XMPP password.
	ix, err := newInternalXMPP(x.jid, password, x.proxyName, x.server, x.port, x.pingTimeout, x.pingInterval, x.notifications, x.dead)
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
			log.Error("XMPP conversation died; restarting")
			if err := x.startXMPP(); err != nil {
				for err != nil {
					log.Errorf("XMPP restart failed, will try again in 10s: %s", err)
					time.Sleep(10 * time.Second)
					err = x.startXMPP()
				}
				log.Error("XMPP conversation restarted successfully")
			}

		case <-x.quit:
			// Close XMPP.
			x.ix.Quit()
			return
		}
	}
}
