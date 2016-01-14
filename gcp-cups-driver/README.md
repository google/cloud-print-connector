This driver is experimental.

Copy backend-local to /usr/libexec/cups/backend/gcp-local

To build the PPD files:

```
ppdc driver.drv
```

Copy driver.drv (not the PPDs) to /usr/share/cups/drv. This doesn't work currently due to SIP:
https://en.wikipedia.org/wiki/System_Integrity_Protection
