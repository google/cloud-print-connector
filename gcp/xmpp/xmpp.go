// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Google Talk XMPP interface, for Cloud Print.
// Forked from Russ Cox's XMPP implementation.
package xmpp

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/golang/glog"
)

const (
	nsStream  = "http://etherx.jabber.org/streams"
	nsSASL    = "urn:ietf:params:xml:ns:xmpp-sasl"
	nsBind    = "urn:ietf:params:xml:ns:xmpp-bind"
	nsSession = "urn:ietf:params:xml:ns:xmpp-session"
	nsClient  = "jabber:client"

	xmppHost = "talk.google.com"
	xmppPort = "443"
)

type XMPP struct {
	t       *tls.Conn
	xd      *xml.Decoder
	fullJID string
	bareJID string
}

func NewXMPP(xmppJID, accessToken string) (*XMPP, error) {
	var user, domain string
	if a := strings.SplitN(xmppJID, "@", 2); len(a) != 2 {
		return nil, errors.New("xmpp: invalid XMPP JID (want user@domain): " + xmppJID)
	} else {
		user = a[0]
		domain = a[1]
	}

	x := XMPP{}

	if err := x.dial(); err != nil {
		x.Close()
		return nil, err
	}
	if err := x.saslHandshake(user, accessToken, domain); err != nil {
		x.Close()
		return nil, err
	}
	if err := x.xmppHandshake(domain); err != nil {
		x.Close()
		return nil, err
	}
	if err := x.gcpHandshake(); err != nil {
		x.Close()
		return nil, err
	}

	return &x, nil
}

func (x *XMPP) Close() error {
	return x.t.Close()
}

func (x *XMPP) dial() error {
	c, err := net.Dial("tcp", net.JoinHostPort(xmppHost, xmppPort))
	if err != nil {
		return err
	}

	x.t = tls.Client(c, &tls.Config{ServerName: xmppHost})

	if err = x.t.Handshake(); err != nil {
		return err
	}
	if err = x.t.VerifyHostname(xmppHost); err != nil {
		return err
	}
	if _, err = x.t.Write([]byte(xml.Header)); err != nil {
		return err
	}
	x.xd = xml.NewDecoder(x.t)

	return nil
}

func (x *XMPP) saslHandshake(user, passwd, domain string) error {
	// Declare intent to be a jabber client.
	fmt.Fprintf(x.t, "<stream:stream to='%s' xmlns='%s' xmlns:stream='%s' version='1.0' xml:lang='en'>",
		domain, nsClient, nsStream)

	// Server should respond with a stream opening.
	se, err := nextStart(x.xd)
	if err != nil {
		return err
	}
	if se.Name.Space != nsStream || se.Name.Local != "stream" {
		return errors.New("xmpp: expected <stream> but got <" + se.Name.Local + "> in " + se.Name.Space)
	}

	// Now we're in the stream and can use Unmarshal.
	// Next message should be <features> to tell us authentication options.
	// See section 4.6 in RFC 3920.
	var f streamFeatures
	if err = x.xd.Decode(&f); err != nil {
		return errors.New("unmarshal <features>: " + err.Error())
	}

	// OAuth2 authentication: send base64-encoded \x00 user \x00 token.
	raw := "\x00" + user + "\x00" + passwd
	enc := base64.StdEncoding.EncodeToString([]byte(raw))
	fmt.Fprintf(x.t, "<auth xmlns='%s' mechanism='X-OAUTH2' auth:service='chromiumsync' auth:allow-generated-jid='true' auth:client-uses-full-bind-result='true' xmlns:auth='http://www.google.com/talk/protocol/auth'>%s</auth>",
		nsSASL, enc)

	// Next message should be either success or failure.
	name, val, err := next(x.xd)
	switch v := val.(type) {
	case *saslSuccess:
	case *saslFailure:
		// v.Any is type of sub-element in failure,
		// which gives a description of what failed.
		return errors.New("auth failure: " + v.Any.Local)
	default:
		return errors.New("expected <success> or <failure>, got <" + name.Local + "> in " + name.Space)
	}

	return nil
}

func (x *XMPP) xmppHandshake(domain string) error {
	// Declare intent to be a jabber client.
	fmt.Fprintf(x.t, "<stream:stream to='%s' xmlns='%s' xmlns:stream='%s' version='1.0' xml:lang='en'>",
		domain, nsClient, nsStream)

	// Here comes another <stream> and <features>.
	se, err := nextStart(x.xd)
	if err != nil {
		return err
	}
	if se.Name.Space != nsStream || se.Name.Local != "stream" {
		return errors.New("expected <stream>, got <" + se.Name.Local + "> in " + se.Name.Space)
	}
	var f streamFeatures
	if err = x.xd.Decode(&f); err != nil {
		return err
	}

	// Send IQ message asking to bind to the local user name.
	fmt.Fprintf(x.t, "<iq type='set' id='0'><bind xmlns='%s'/></iq>\n", nsBind)
	var iq clientIQ
	if err = x.xd.Decode(&iq); err != nil {
		return errors.New("unmarshal <iq>: " + err.Error())
	}
	if iq.Bind == nil {
		return errors.New("<iq> result missing <bind>")
	}
	x.fullJID = iq.Bind.Jid // our local id

	fmt.Fprintf(x.t, "<iq type='set' id='1'><session xmlns='%s'/></iq>", nsSession)
	if err = x.xd.Decode(&iq); err != nil {
		return errors.New("unmarshal <iq>: " + err.Error())
	}

	i := strings.Index(x.fullJID, "/")
	if i >= 0 {
		x.bareJID = x.fullJID[:i]
	} else {
		x.bareJID = x.fullJID
	}

	return nil
}

func (x *XMPP) gcpHandshake() error {
	fmt.Fprintf(x.t,
		"<iq type='set' to='%s' id='3'><subscribe xmlns='google:push'><item channel='cloudprint.google.com' from='cloudprint.google.com'/></subscribe></iq>",
		x.bareJID)

	var iq clientIQ
	if err := x.xd.Decode(&iq); err != nil {
		return errors.New("unmarshal <iq>: " + err.Error())
	}
	if iq.To != x.fullJID || iq.From != x.bareJID {
		return errors.New("<iq> missing bare JID confirmation")
	}

	return nil
}

// Blocks until a printer has received a job, then returns the GCPID of that printer.
func (x *XMPP) NextWaitingPrinter() (string, error) {
	for {
		_, val, err := next(x.xd)
		if err != nil {
			return "", err
		}
		switch v := val.(type) {
		case *clientMessage:
			return v.GCPID, nil
		default:
		}
	}
}

// RFC 3920  C.1  Streams name space

type streamFeatures struct {
	XMLName    xml.Name `xml:"http://etherx.jabber.org/streams features"`
	Mechanisms saslMechanisms
	Bind       bindBind
	Session    bool
}

// RFC 3920  C.4  SASL name space

type saslMechanisms struct {
	XMLName   xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-sasl mechanisms"`
	Mechanism []string `xml:"mechanism"`
}

type saslSuccess struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-sasl success"`
}

type saslFailure struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-sasl failure"`
	Any     xml.Name
}

// RFC 3920  C.5  Resource binding name space

type bindBind struct {
	XMLName  xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-bind bind"`
	Resource string   `xml:"resource"`
	Jid      string   `xml:"jid"`
}

// RFC 3921  B.1  jabber:client

type clientMessage struct {
	XMLName xml.Name `xml:"jabber:client message"`
	GCPID   string   `xml:"push>data"`
}

type clientIQ struct { // info/query
	XMLName xml.Name `xml:"jabber:client iq"`
	From    string   `xml:"from,attr"`
	Id      string   `xml:"id,attr"`
	To      string   `xml:"to,attr"`
	Type    string   `xml:"type,attr"` // error, get, result, set
	Error   clientError
	Bind    *bindBind `xml:"bind"`
}

type clientError struct {
	XMLName xml.Name `xml:"jabber:client error"`
	Code    string   `xml:",attr"`
	Type    string   `xml:",attr"`
	Any     xml.Name
	Text    string
}

// Scan XML token stream to find next StartElement.
func nextStart(p *xml.Decoder) (xml.StartElement, error) {
	for {
		t, err := p.Token()
		if err != nil {
			glog.Fatal("token", err)
		}
		switch t := t.(type) {
		case xml.StartElement:
			return t, nil
		}
	}
	panic("unreachable")
}

// Scan XML token stream for next element and save into val.
// If val == nil, allocate new element based on proto map.
// Either way, return val.
func next(p *xml.Decoder) (xml.Name, interface{}, error) {
	// Read start element to find out what type we want.
	se, err := nextStart(p)
	if err != nil {
		return xml.Name{}, nil, err
	}

	// Put it in an interface and allocate one.
	var nv interface{}
	switch se.Name.Space + " " + se.Name.Local {
	case nsStream + " features":
		nv = &streamFeatures{}
	case nsSASL + " mechanisms":
		nv = &saslMechanisms{}
	case nsSASL + " challenge":
		nv = ""
	case nsSASL + " response":
		nv = ""
	case nsSASL + " success":
		nv = &saslSuccess{}
	case nsSASL + " failure":
		nv = &saslFailure{}
	case nsBind + " bind":
		nv = &bindBind{}
	case nsClient + " message":
		nv = &clientMessage{}
	case nsClient + " iq":
		nv = &clientIQ{}
	case nsClient + " error":
		nv = &clientError{}
	default:
		return xml.Name{}, nil, errors.New("unexpected XMPP message " +
			se.Name.Space + " <" + se.Name.Local + "/>")
	}

	// Unmarshal into that storage.
	if err = p.DecodeElement(nv, &se); err != nil {
		return xml.Name{}, nil, err
	}
	return se.Name, nv, err
}

type tee struct {
	r io.Reader
}

func (t tee) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	fmt.Printf("read %d bytes", n)
	if n > 0 {
		fmt.Printf(" %s\n", p[0:n])
	} else {
		fmt.Printf("\n")
	}
	return
}
