/*
Copyright 2016 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"
)

// portManager opens ports within the interval [min, max], starting with min.
type portManager struct {
	min uint16
	max uint16
}

var NoPortsAvailable = errors.New("No ports available")

// listen finds an open port, returns an open listener on that port.
//
// Returns error when no ports are available.
func (p *portManager) listen() (*quittableListener, error) {
	for port := p.min; port <= p.max; port++ {
		if l, err := newQuittableListener(port); err == nil {
			return l, nil
		} else {
			if !isAddrInUse(err) {
				return nil, err
			}
		}
	}
	return nil, NoPortsAvailable
}

func isAddrInUse(err error) bool {
	if err, ok := err.(*net.OpError); ok {
		if err, ok := err.Err.(*os.SyscallError); ok {
			return err.Err == syscall.EADDRINUSE
		}
	}
	return false
}

type quittableListener struct {
	*net.TCPListener
	// When q is closed, the listener is quitting.
	q chan struct{}
}

func newQuittableListener(port uint16) (*quittableListener, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(port)})
	if err != nil {
		return nil, err
	}
	return &quittableListener{l, make(chan struct{}, 0)}, nil
}

func (l *quittableListener) Accept() (net.Conn, error) {
	conn, err := l.AcceptTCP()

	select {
	case <-l.q:
		if err == nil {
			conn.Close()
		}
		// The listener was closed on purpose.
		// Returning an error that is not a net.Error causes net.Server.Serve() to return.
		return nil, closed
	default:
	}

	// Clean up zombie connections.
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Minute)

	return conn, err
}

func (l *quittableListener) quit() {
	close(l.q)
	l.Close()
}
