/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package gcp

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	// Dump XMPP XMP conversation to stdout.
	debug = false

	// This is a long-lived, potentially quiet, conversation. Keep it alive!
	netKeepAlive = time.Second * 60

	// Set our own timeout, rather than have the OS or server timeout for us.
	netTimeout = time.Second * 60
)

// Indicates a closed connection; we're probably exiting.
var ErrClosed = errors.New("closed")

// Interface with XMPP server.
type gcpXMPP struct {
	conn       *tls.Conn
	xmlDecoder *xml.Decoder
}

type nextPrinterResponse struct {
	gcpID string
	err   error
}

func newXMPP(xmppJID, accessToken, proxyName string) (*gcpXMPP, error) {
	var user, domain string
	if parts := strings.SplitN(xmppJID, "@", 2); len(parts) != 2 {
		return nil, fmt.Errorf("Tried to use invalid XMPP JID: %s", xmppJID)
	} else {
		user = parts[0]
		domain = parts[1]
	}

	// Anyone home?
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("Failed to dial XMPP service: %s", err)
	}

	var xmlEncoder *xml.Encoder
	var xmlDecoder *xml.Decoder
	if debug {
		t := &tee{conn, conn}
		xmlEncoder = xml.NewEncoder(t)
		xmlDecoder = xml.NewDecoder(t)
	} else {
		xmlEncoder = xml.NewEncoder(conn)
		xmlDecoder = xml.NewDecoder(conn)
	}

	// SASL
	if err := saslHandshake(xmlEncoder, xmlDecoder, domain, user, accessToken); err != nil {
		return nil, fmt.Errorf("Failed to perform XMPP-SASL handshake: %s", err)
	}

	// XMPP
	fullJID, err := xmppHandshake(xmlEncoder, xmlDecoder, domain, proxyName)
	if err != nil {
		return nil, fmt.Errorf("Failed to perform final XMPP handshake: %s", err)
	}

	// Subscribe
	if err := subscribe(xmlEncoder, xmlDecoder, fullJID); err != nil {
		return nil, fmt.Errorf("Failed to subscribe: %s", err)
	}

	return &gcpXMPP{conn, xmlDecoder}, nil
}

// nextWaitingPrinter returns the GCPID of the next printer with waiting jobs.
func (x *gcpXMPP) nextWaitingPrinter() (string, error) {
	// First, verify that this is a message we expect to see.
	startElement, err := readStartElement(x.xmlDecoder)
	if err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return "", ErrClosed
		}
		return "", fmt.Errorf("Failed to read the next start element: %s", err)
	}
	if startElement.Name.Local != "message" {
		return "", fmt.Errorf("Unexpected element while waiting for print message: %+v", startElement)
	}

	// Second, parse the message.
	var message struct {
		XMLName xml.Name `xml:"message"`
		Data    string   `xml:"push>data"`
	}

	if err := x.xmlDecoder.DecodeElement(&message, startElement); err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return "", ErrClosed
		}
		return "", fmt.Errorf("Error while waiting for print jobs via XMPP: %s", err)
	} else {
		return message.Data, nil
	}
}

func (x *gcpXMPP) quit() {
	x.conn.Close()
}

func dial() (*tls.Conn, error) {
	tlsConfig := &tls.Config{
		ServerName: "talk.google.com",
	}
	netDialer := &net.Dialer{
		KeepAlive: netKeepAlive,
		Timeout:   netTimeout,
	}
	conn, err := tls.DialWithDialer(netDialer, "tcp", "talk.google.com:443", tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to XMPP server: %s", err)
	}
	if err = conn.VerifyHostname("talk.google.com"); err != nil {
		return nil, fmt.Errorf("Failed to verify hostname of XMPP server: %s", err)
	}

	return conn, nil
}

func saslHandshake(xmlEncoder *xml.Encoder, xmlDecoder *xml.Decoder, domain, user, accessToken string) error {
	handshake := xml.StartElement{
		Name: xml.Name{"jabber:client", "stream:stream"},
		Attr: []xml.Attr{
			xml.Attr{xml.Name{Local: "to"}, domain},
			xml.Attr{xml.Name{Local: "xml:lang"}, "en"},
			xml.Attr{xml.Name{Local: "version"}, "1.0"},
			xml.Attr{xml.Name{Local: "xmlns:stream"}, "http://etherx.jabber.org/streams"},
		},
	}
	if err := xmlEncoder.EncodeToken(handshake); err != nil {
		return fmt.Errorf("Failed to write SASL handshake: %s", err)
	}
	if err := xmlEncoder.Flush(); err != nil {
		return fmt.Errorf("Failed to flush encoding stream: %s", err)
	}

	if startElement, err := readStartElement(xmlDecoder); err != nil {
		return err
	} else if startElement.Name.Space != "http://etherx.jabber.org/streams" ||
		startElement.Name.Local != "stream" {
		return fmt.Errorf("Read unexpected SASL XML stanza: %s", startElement.Name.Local)
	}

	var features struct {
		XMLName    xml.Name `xml:"http://etherx.jabber.org/streams features"`
		Mechanisms *struct {
			XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-sasl mechanisms"`
		}
	}
	if err := xmlDecoder.Decode(&features); err != nil {
		return fmt.Errorf("Read unexpected SASL XML element: %s", err)
	} else if features.Mechanisms == nil {
		return errors.New("SASL mechanisms missing from handshake")
	}

	credential := base64.StdEncoding.EncodeToString([]byte("\x00" + user + "\x00" + accessToken))

	var auth struct {
		XMLName    xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-sasl auth"`
		Mechanism  string   `xml:"mechanism,attr"`
		Service    string   `xml:"auth:service,attr"`
		Allow      string   `xml:"auth:allow-generated-jid,attr"`
		FullBind   string   `xml:"auth:client-uses-full-bind-result,attr"`
		XMLNS      string   `xml:"xmlns:auth,attr"`
		Credential string   `xml:",chardata"`
	}
	auth.Mechanism = "X-OAUTH2"
	auth.Service = "chromiumsync"
	auth.Allow = "true"
	auth.FullBind = "true"
	auth.XMLNS = "http://www.google.com/talk/protocol/auth"
	auth.Credential = credential
	if err := xmlEncoder.Encode(auth); err != nil {
		return fmt.Errorf("Failed to write SASL credentials: %s", err)
	}

	var success struct {
		XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-sasl success"`
	}
	if err := xmlDecoder.Decode(&success); err != nil {
		return fmt.Errorf("Failed to complete SASL handshake: %s", err)
	}

	return nil
}

func xmppHandshake(xmlEncoder *xml.Encoder, xmlDecoder *xml.Decoder, domain, proxyName string) (string, error) {
	handshake := xml.StartElement{
		Name: xml.Name{"jabber:client", "stream:stream"},
		Attr: []xml.Attr{
			xml.Attr{xml.Name{Local: "to"}, domain},
			xml.Attr{xml.Name{Local: "xml:lang"}, "en"},
			xml.Attr{xml.Name{Local: "version"}, "1.0"},
			xml.Attr{xml.Name{Local: "xmlns:stream"}, "http://etherx.jabber.org/streams"},
		},
	}
	if err := xmlEncoder.EncodeToken(handshake); err != nil {
		return "", fmt.Errorf("Failed to write SASL handshake: %s", err)
	}
	if err := xmlEncoder.Flush(); err != nil {
		return "", fmt.Errorf("Failed to flush encoding stream: %s", err)
	}

	if startElement, err := readStartElement(xmlDecoder); err != nil {
		return "", err
	} else if startElement.Name.Space != "http://etherx.jabber.org/streams" ||
		startElement.Name.Local != "stream" {
		return "", fmt.Errorf("Read unexpected XMPP XML stanza: %s", startElement.Name.Local)
	}

	var features struct {
		XMLName xml.Name `xml:"http://etherx.jabber.org/streams features"`
		Bind    *struct {
			XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-bind bind"`
		}
		Session *struct {
			XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-session session"`
		}
	}
	if err := xmlDecoder.Decode(&features); err != nil {
		return "", fmt.Errorf("Read unexpected XMPP XML element: %s", err)
	} else if features.Bind == nil || features.Session == nil {
		return "", errors.New("XMPP bind or session missing from handshake")
	}

	var resource struct {
		XMLName xml.Name `xml:"jabber:client iq"`
		Type    string   `xml:"type,attr"`
		ID      string   `xml:"id,attr"`
		Bind    struct {
			XMLName  xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-bind bind"`
			Resource struct {
				XMLName      xml.Name `xml:"resource"`
				ResourceName string   `xml:",chardata"`
			}
		}
	}
	resource.Type = "set"
	resource.ID = "0"
	resource.Bind.Resource.ResourceName = proxyName
	if err := xmlEncoder.Encode(&resource); err != nil {
		return "", fmt.Errorf("Failed to set resource during XMPP handshake: %s", err)
	}

	var jid struct {
		XMLName xml.Name `xml:"jabber:client iq"`
		Bind    *struct {
			XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-bind bind"`
			JID     string   `xml:"jid"`
		}
	}
	if err := xmlDecoder.Decode(&jid); err != nil {
		return "", err
	} else if jid.Bind == nil || jid.Bind.JID == "" {
		return "", errors.New("Received unexpected XML element during XMPP handshake")
	}

	fullJID := jid.Bind.JID

	var session struct {
		XMLName xml.Name `xml:"jabber:client iq"`
		Type    string   `xml:"type,attr"`
		ID      string   `xml:"id,attr"`
		Session struct {
			XMLName xml.Name `xml:"urn:ietf:params:xml:ns:xmpp-session session"`
		}
	}
	session.Type = "set"
	session.ID = "1"
	if err := xmlEncoder.Encode(&session); err != nil {
		return "", fmt.Errorf("Failed to complete XMPP handshake: %s", err)
	}

	var xmppDone struct {
		XMLName xml.Name `xml:"jabber:client iq"`
		ID      string   `xml:"id,attr"`
	}
	if err := xmlDecoder.Decode(&xmppDone); err != nil {
		return "", err
	} else if xmppDone.ID != "1" {
		return "", errors.New("Received unexpected result at end of XMPP handshake")
	}

	return fullJID, nil
}

func subscribe(xmlEncoder *xml.Encoder, xmlDecoder *xml.Decoder, fullJID string) error {
	var bareJID string
	if barePosition := strings.Index(fullJID, "/"); barePosition < 0 {
		return fmt.Errorf("Can't split JID %s", fullJID)
	} else {
		bareJID = fullJID[:barePosition]
	}

	var subscribe struct {
		XMLName   xml.Name `xml:"jabber:client iq"`
		Type      string   `xml:"type,attr"`
		To        string   `xml:"to,attr"`
		ID        string   `xml:"id,attr"`
		Subscribe struct {
			XMLName xml.Name `xml:"google:push subscribe"`
			Item    struct {
				XMLName xml.Name `xml:"item"`
				Channel string   `xml:"channel,attr"`
				From    string   `xml:"from,attr"`
			}
		}
	}
	subscribe.Type = "set"
	subscribe.To = bareJID
	subscribe.ID = "3"
	subscribe.Subscribe.Item.Channel = "cloudprint.google.com"
	subscribe.Subscribe.Item.From = "cloudprint.google.com"
	if err := xmlEncoder.Encode(&subscribe); err != nil {
		return fmt.Errorf("XMPP subscription request failed: %s", err)
	}

	var subscription struct {
		XMLName xml.Name `xml:"jabber:client iq"`
		To      string   `xml:"to,attr"`
		From    string   `xml:"from,attr"`
	}
	if err := xmlDecoder.Decode(&subscription); err != nil {
		return fmt.Errorf("XMPP subscription response invalid: %s", err)
	} else if fullJID != subscription.To || bareJID != subscription.From {
		return errors.New("XMPP subscription failed")
	}

	return nil
}

func readStartElement(d *xml.Decoder) (*xml.StartElement, error) {
	for {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		if startElement, ok := token.(xml.StartElement); ok {
			return &startElement, nil
		}
	}
	panic("unreachable")
}

type tee struct {
	r io.Reader
	w io.Writer
}

func (t *tee) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	fmt.Printf("read %d %s\n", n, p[0:n])
	return n, err
}

func (t *tee) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	fmt.Printf("wrote %d %s\n", n, p[0:n])
	return n, err
}
