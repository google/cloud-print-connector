# Google Cloud Print CUPS Connector
The Google Cloud Print (aka GCP) CUPS Connector shares CUPS printers with GCP.

# License
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

# Install
```
$ cd ~/go/src
$ go get TODO/cups-connector
$ go get github.com/golang/oauth2
$ go get github.com/golang/glog
$ cd cups-connector/connector
$ go install
$ cd ../connector-init
$ go install
$ cd ../connector-monitor
$ go install
```

## Configure
To create a basic config file called `cups-connector.config.json`, use
`connector-init`. The default config file looks something like this; make sure
the `/var/run/cups-connector` directory exists and is writeable by the
connector:

```
{
  "xmpp_jid": "e73b3deadc7bbbeefc1d2d22@cloudprint.googleusercontent.com",
  "robot_refresh_token": "1/D39yourG_KMbeefjnsis1peMIp5DeadMyOkwOQMZhSo",
  "user_refresh_token": "1/fBXneverhZHieath_2an2UxDVsourGE8pwatermelon",
  "share_scope": "somedude@gmail.com",
  "proxy_name": "joes-crab-shack",
  "gcp_max_concurrent_downloads": 5,
  "cups_job_queue_size": 3,
  "cups_printer_poll_interval": "1m0s",
  "cups_printer_attributes": [
    "printer-name",
    "printer-info",
    "printer-is-accepting-jobs",
    "printer-location",
    "printer-make-and-model",
    "printer-state",
    "printer-state-reasons"
  ],
  "cups_job_full_username": false,
  "cups_ignore_raw_printers": true,
  "copy_printer_info_to_display_name": true,
  "monitor_socket_filename": "/var/run/cups-connector/monitor.sock"
}
```

To fetch all printer attributes (there are lots), use a single attribute named
`"all"`.
```
{
...
  "cups_printer_attributes": [
    "all",
  ],
...
}
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
variables. To help keep track of these kinds of things, C variables in the CUPS
package are named c_foo.

The connector uses the GCP 1.0 API. At the time that the connector was written,
1.0 was easier and faster to implement, plus the 2.0 API didn't provide upsides
material to the connector.
