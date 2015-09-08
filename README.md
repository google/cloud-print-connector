# Google Cloud Print CUPS Connector
The Google Cloud Print (aka GCP) CUPS Connector shares CUPS printers with users of Google Cloud Print.

# License
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd

# Install

### Get Ready
Get the most recent version of the Go compiler: https://golang.org/doc/install

The CUPS Connector also uses some C libraries. Get the necessary build tools and libraries for your platform:

#### Debian, Ubuntu, Raspberry Pi and friends
Make sure you are running CUPS 1.7 or later. If using the distribution-bundled CUPS package, upgrade to Debian >= 8.0 jessie or Ubuntu >= 14.04 trusty.

A special note to Raspbian users: Although 8.0 jessie isn't mentioned at raspbian.org, it is possible to upgrade: https://raspberrypi.stackexchange.com/questions/27858/upgrade-to-raspbian-jessie

Ready? Install build tools and libraries:
```
$ sudo apt-get install build-essential libcups2-dev libsnmp-dev libavahi-client-dev
```

#### OS X
Install XCode: https://itunes.apple.com/us/app/xcode/id497799835

Install the command line developer tools:
```
$ xcode-select --install
```

Accept the license agreement:
```
$ xcodebuild -license
```

#### Other platforms
Any Linux distribution or *BSD flavor _should_ support the CUPS Connector. If you have trouble (or success!) with another platform, please open an issue so that we can integrate the feedback here.

### Install the Connector
```
$ go get github.com/google/cups-connector/...
```

Once installation completes, four binaries will be placed in $GOPATH/bin.

binary              | purpose
------------------- | -------
`connector`         | Runs for long periods of time, shares CUPS printers, processes print jobs.
`connector-init`    | Handy tool to create a new config file.
`connector-monitor` | Gathers various information about the running connector, reports results to stdout.
`connector-util`    | Tool to upgrade a config file after a release, delete all printers, future tasks.

### Configure the Connector
To create a config file called `cups-connector.config.json`, run
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
  "gcp_oauth_token_url": "https://accounts.google.com/o/oauth2/token",
  "snmp_enable": true,
  "snmp_community": "public",
  "snmp_max_connections": 100
}
```

### Prepare monitor socket directory
Make sure that the socket directory (see `monitor_socket_filename` above),
exists and is writeable by the user that the connector will run as:
```
$ sudo mkdir /var/run/cups-connector
$ sudo chown $USER /var/run/cups-connector
```

Of course, you'll have to do this every time the platform boots, because
`/var/run` isn't persistent. If you prefer to keep things simple, then forget
what I said before about `mkdir` and `chown`, and change the config file value for
`monitor_socket_filename` to `/tmp/cups-connector-monitor.sock`.

### Configure CUPS client => server conversation
Your platform is probably configured to talk to the CUPS server on localhost,
and that's probably what you want. If not, this next part is for you.

When deciding which CUPS server to connect to, the connector asks the CUPS client
library, specifically the
[cupsServer()](https://www.cups.org/documentation.php/doc-2.0/api-cups.html#cupsServer)
and
[cupsEncryption()](https://www.cups.org/documentation.php/doc-2.0/api-cups.html#cupsEncryption)
functions, which return values found in:
- CUPS_SERVER and CUPS_ENCRYPTION environment variables
- ~/.cups/client.conf
- /etc/cups/client.conf

### Start the Connector automatically
The simplest way to start the connector on boot is to edit `/etc/rc.local`.
Add the following lines before `exit 0`. The example user is "pi", which
you should change to your own username:

```
# CUPS Connector:
#   su ... pi: run as user "pi"
#   --login:   environment similar to "pi" instead of "root"
#   --command: run this thing
#   &:         run the command "in the background"
su --login --command "go/bin/connector" pi &
```

### Firewall Requirements
In order for the connector to function properly, it needs to make the following connections:
- accounts.google.com - HTTPS port 443 (OAuth authorize and token refresh)
- www.google.com - HTTPS port 443 (/cloudprint API endpoints)
- talk.google.com - XMPP port 443 (print job notification channel)

you need to ensure your firewall allows connections to each of these hosts.
