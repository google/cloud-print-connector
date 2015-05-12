# Google Cloud Print CUPS Connector
The Google Cloud Print (aka GCP) CUPS Connector shares CUPS printers with GCP.

# License
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd

# Install
Get a recent version of the Go compiler. This is best done without your operating system's
package manager: https://golang.org/doc/install

You'll need the CUPS development libraries. Debian, Ubuntu, etc:
```
$ sudo apt-get install libcups2-dev
```

We use a little bit of C to marry the CUPS client library to Go code. Debian, Ubuntu, etc:
```
$ sudo apt-get install build-essential
```

Install the Connector:
```
$ go get github.com/google/cups-connector/connector
$ go get github.com/google/cups-connector/connector-init
$ go get github.com/google/cups-connector/connector-monitor
$ go get github.com/google/cups-connector/connector-util
```

# Configure
To create a basic config file called `cups-connector.config.json`, use
`connector-init`. The default config file looks something like this:

```
{
  "xmpp_jid": "e73b3deadc7bbbeefc1d2d22@cloudprint.googleusercontent.com",
  "robot_refresh_token": "1/D39yourG_KMbeefjnsis1peMIp5DeadMyOkwOQMZhSo",
  "user_refresh_token": "1/fBXneverhZHieath_2an2UxDVsourGE8pwatermelon",
  "share_scope": "somedude@gmail.com",
  "proxy_name": "joes-crab-shack",
  "gcp_max_concurrent_downloads": 5,
  "cups_max_connections": 5,
  "cups_connect_timeout": "5s",
  "cups_job_queue_size": 3,
  "cups_printer_poll_interval": "1m",
  "cups_printer_attributes": [
    "printer-name",
    "printer-info",
    "printer-location",
    "printer-make-and-model",
    "printer-state",
    "printer-state-reasons",
    "printer-uuid",
    "marker-names",
    "marker-types",
    "marker-levels"
  ],
  "cups_job_full_username": false,
  "cups_ignore_raw_printers": true,
  "copy_printer_info_to_display_name": true,
  "monitor_socket_filename": "/var/run/cups-connector/monitor.sock",
  "gcp_base_url": "https://www.google.com/cloudprint/",
  "xmpp_server": "talk.google.com",
  "xmpp_port": 443,
  "gcp_xmpp_ping_timeout": "5s",
  "gcp_xmpp_ping_interval_default": "2m",
  "gcp_oauth_client_id": "539833558011-35iq8btpgas80nrs3o7mv99hm95d4dv6.apps.googleusercontent.com",
  "gcp_oauth_client_secret": "V9BfPOvdiYuw12hDx5Y5nR0a",
  "gcp_oauth_auth_url": "https://accounts.google.com/o/oauth2/auth",
  "gcp_oauth_token_url": "https://accounts.google.com/o/oauth2/token"
}
```

Finally, make sure that the socket directory (see `monitor_socket_filename` above),
exists and is writeable by the user that the connector will run as:
```
$ sudo mkdir /var/run/cups-connector
$ sudo chown $USER /var/run/cups-connector
```

## Configure CUPS client:server
When deciding which CUPS server to connect to, the connector uses the standard
CUPS client library, specifically the
[cupsServer()](https://www.cups.org/documentation.php/doc-1.7/api-cups.html#cupsServer)
and
[cupsEncryption()](https://www.cups.org/documentation.php/doc-1.7/api-cups.html#cupsEncryption)
functions, which return values found in:
- CUPS_SERVER and CUPS_ENCRYPTION environment variables
- ~/.cups/client.conf
- /etc/cups/client.conf

# About the code
CUPS is a mature service. When given the choice to check queues, timeouts,
retries, etc, the connector assumes that the CUPS service will do it's job.
When empirical evidence contradicts this assumption, checks are added.

The CUPS package uses cgo to access the CUPS/IPP API. This results in lots of
C variables, mixed in with golang variables. C variables often require explicit
memory freeing, and often represent the same data as neighboring golang
variables.
