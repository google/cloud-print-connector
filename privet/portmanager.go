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
	"sync"
	"syscall"
	"time"
)

var NoPortsAvailable = errors.New("No ports available")

// portManager opens ports within the interval [low, high], starting with low.
type portManager struct {
	low  uint16
	high uint16

	// Keeping a cache of used ports improves benchmark tests by over 100x.
	m sync.Mutex
	p map[uint16]struct{}
}

func newPortManager(low, high uint16) *portManager {
	return &portManager{
		low:  low,
		high: high,
		p:    make(map[uint16]struct{}),
	}
}

// listen finds an open port, returns an open listener on that port.
//
// Returns error when no ports are available.
func (p *portManager) listen() (*quittableListener, error) {
	for port := p.nextAvailablePort(p.low); port != 0; port = p.nextAvailablePort(port) {
		if l, err := newQuittableListener(port, p); err == nil {
			return l, nil
		} else {
			if !isAddrInUse(err) {
				return nil, err
			}
		}
	}

	return nil, NoPortsAvailable
}

// nextAvailablePort checks the p map for the next port available.
// p only keeps track of ports used by the connector, so the start parameter
// is useful to check the port after a port that is in use by some other process.
//
// Returns zero when no available port can be found.
func (p *portManager) nextAvailablePort(start uint16) uint16 {
	p.m.Lock()
	defer p.m.Unlock()

	for port := start; port <= p.high; port++ {
		if _, exists := p.p[port]; !exists {
			p.p[port] = struct{}{}
			return port
		}
	}

	return 0
}

func (p *portManager) freePort(port uint16) {
	p.m.Lock()
	defer p.m.Unlock()

	delete(p.p, port)
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

	pm *portManager

	// When q is closed, the listener is quitting.
	q chan struct{}
}

func newQuittableListener(port uint16, pm *portManager) (*quittableListener, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(port)})
	if err != nil {
		return nil, err
	}
	return &quittableListener{l, pm, make(chan struct{}, 0)}, nil
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

func (l *quittableListener) Close() error {
	err := l.TCPListener.Close()
	if err != nil {
		return err
	}
	l.pm.freePort(l.port())
	return nil
}

func (l *quittableListener) port() uint16 {
	return uint16(l.Addr().(*net.TCPAddr).Port)
}

func (l *quittableListener) quit() {
	close(l.q)
	l.Close()
}
