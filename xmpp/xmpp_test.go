/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package xmpp_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/cups-connector/xmpp"
)

func TestXMPP_proxyauth(t *testing.T) {
	cfg := configureTLS(t)
	ts := httptest.NewServer(&testXMPPHandler{T: t, cfg: cfg, wantProxyAuth: "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ=="})
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal("failed to parse URL", ts.URL)
	}
	u.User = url.UserPassword("Aladdin", "open sesame")

	orig := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		Proxy:           http.ProxyURL(u),
		TLSClientConfig: cfg,
	}
	defer func() {
		http.DefaultTransport = orig
	}()

	strs := strings.Split(u.Host, ":")
	port, err := strconv.Atoi(strs[1])
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan<- xmpp.PrinterNotification)
	x, err := xmpp.NewXMPP("jid@example.com", "proxyName", strs[0], uint16(port), time.Minute, time.Minute, func() (string, error) {
		return "accessToken", nil
	}, ch)
	if err != nil {
		t.Fatal(err)
	}
	x.Quit()
}

type testXMPPHandler struct {
	*testing.T
	cfg           *tls.Config
	wantProxyAuth string

	dec *xml.Decoder
}

func (t testXMPPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "CONNECT" {
		t.Fatal("want: proxy CONNECT but:", r.Method)
	}

	if auth := r.Header.Get("Proxy-Authorization"); auth != t.wantProxyAuth {
		t.Fatal("want: ", t.wantProxyAuth, " but: ", auth)
	}
	w.WriteHeader(http.StatusOK)

	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("webserver doesn't support hijacking")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		t.Fatal("failed to hijack", err)
	}
	defer conn.Close()

	if err := bufrw.Flush(); err != nil {
		t.Fatal("failed to flush", err)
	}

	cloneTLSConfig := *t.cfg
	tlsConn := tls.Server(conn, &cloneTLSConfig)

	if err := tlsConn.Handshake(); err != nil {
		t.Fatal("failed to handshake TLS", err)
	}

	t.serveXMPP(tlsConn)
}

func (t testXMPPHandler) serveXMPP(conn net.Conn) {
	t.dec = xml.NewDecoder(conn)
	t.xmppHello(conn)
	// from https://developers.google.com/cloud-print/docs/rawxmpp
	t.saslHandshake(conn)
	t.xmppHandshake(conn)
	t.handleSubscribe(conn)
	t.handlePing(conn)
	ioutil.ReadAll(conn) // wait for client end
}

func (t testXMPPHandler) xmppHello(conn net.Conn) {
	io.WriteString(conn, `
<?xml version='1.0'?>
`)
}

func (t testXMPPHandler) saslHandshake(conn net.Conn) {
	t.readElement("stream")
	io.WriteString(conn, `
<stream:stream from="gmail.com" id="1" version="1.0" xmlns:stream="http://etherx.jabber.org/streams" xmlns="jabber:client">
<stream:features>
  <mechanisms xmlns="urn:ietf:params:xml:ns:xmpp-sasl">
    <mechanism>PLAIN</mechanism>
    <mechanism>X-GOOGLE-TOKEN</mechanism>
    <mechanism>X-OAUTH2</mechanism>
  </mechanisms>
</stream:features>
`)
	t.readElement("auth")
	io.WriteString(conn, `
<success xmlns="urn:ietf:params:xml:ns:xmpp-sasl"/>
`)
}

func (t testXMPPHandler) xmppHandshake(conn net.Conn) {
	t.readElement("stream")
	io.WriteString(conn, `
<stream:stream from="gmail.com" id="2" version="1.0" xmlns:stream="http://etherx.jabber.org/streams" xmlns="jabber:client">
<stream:features>
  <bind xmlns="urn:ietf:params:xml:ns:xmpp-bind"/>
  <session xmlns="urn:ietf:params:xml:ns:xmpp-session"/>
</stream:features>
`)

	t.readElement("iq", "bind")
	io.WriteString(conn, `
<iq id="0" type="result">
  <bind xmlns="urn:ietf:params:xml:ns:xmpp-bind">
    <jid>barejid/fulljid</jid>
  </bind>
</iq>
`)

	t.readElement("iq", "session")
	io.WriteString(conn, `
<iq type="result" id="1"/>
`)
}

func (t testXMPPHandler) handleSubscribe(conn net.Conn) {
	t.readElement("iq", "subscribe")
	io.WriteString(conn, `
<iq to="barejid/fulljid" from="barejid" id="3" type="result"/>
`)
}

func (t testXMPPHandler) handlePing(conn net.Conn) {
	t.readElement("iq", "ping")
	io.WriteString(conn, `
<iq to="barejid/fulljid" from="cloudprint.google.com" id="0" type="result"/>
`)
}

func (t testXMPPHandler) readElement(wantName string, wantChildren ...string) *xml.StartElement {
	d := t.dec
	for {
		token, err := d.Token()
		if err != nil {
			t.Fatal("failed to read start element", err)
		}
		if startElement, ok := token.(xml.StartElement); ok {
			if actual := startElement.Name.Local; actual != wantName {
				continue
			}
			for _, want := range wantChildren {
				t.readElement(want)
			}
			return &startElement
		}
	}
	panic("unreachable")
}

func configureTLS(t *testing.T) *tls.Config {
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		t.Fatal("failed to load x509 key pair", err)
	}

	cfg := tls.Config{Certificates: []tls.Certificate{cert}}
	x509Cert, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	cfg.RootCAs = x509.NewCertPool()
	cfg.RootCAs.AddCert(x509Cert)
	return &cfg
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs borrowed from http/httptest
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB
iQKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9SjY1bIw4
iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZBl2+XsDul
rKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQABo2gwZjAO
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw
AwEB/zAuBgNVHREEJzAlggtleGFtcGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAA
AAAAATANBgkqhkiG9w0BAQsFAAOBgQCEcetwO59EWk7WiJsG4x8SY+UIAA+flUI9
tyC4lNhbcF2Idq9greZwbYCqTTTr2XiRNSMLCOjKyI7ukPoPjo16ocHj+P3vZGfs
h1fIw3cSS2OolhloGw/XM6RWPWtPAlGykKLciQrBru5NAPvCMsb/I1DAceTiotQM
fblo6RBxUQ==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXgIBAAKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9
SjY1bIw4iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZB
l2+XsDulrKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQAB
AoGAGRzwwir7XvBOAy5tM/uV6e+Zf6anZzus1s1Y1ClbjbE6HXbnWWF/wbZGOpet
3Zm4vD6MXc7jpTLryzTQIvVdfQbRc6+MUVeLKwZatTXtdZrhu+Jk7hx0nTPy8Jcb
uJqFk541aEw+mMogY/xEcfbWd6IOkp+4xqjlFLBEDytgbIECQQDvH/E6nk+hgN4H
qzzVtxxr397vWrjrIgPbJpQvBsafG7b0dA4AFjwVbFLmQcj2PprIMmPcQrooz8vp
jy4SHEg1AkEA/v13/5M47K9vCxmb8QeD/asydfsgS5TeuNi8DoUBEmiSJwma7FXY
fFUtxuvL7XvjwjN5B30pNEbc6Iuyt7y4MQJBAIt21su4b3sjXNueLKH85Q+phy2U
fQtuUE9txblTu14q3N7gHRZB4ZMhFYyDy8CKrN2cPg/Fvyt0Xlp/DoCzjA0CQQDU
y2ptGsuSmgUtWj3NM9xuwYPm+Z/F84K6+ARYiZ6PYj013sovGKUFfYAqVXVlxtIX
qyUBnu3X9ps8ZfjLZO7BAkEAlT4R5Yl6cGhaJQYZHOde3JEMhNRcVFMO8dJDaFeo
f9Oeos0UUothgiDktdQHxdNEwLjQf7lJJBzV+5OtwswCWA==
-----END RSA PRIVATE KEY-----`)
