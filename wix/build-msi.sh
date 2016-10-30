if [ $# -eq 0 ]; then
  me=$(basename $0)
  echo "Usage: $me <version>"
  exit 1
fi
CONNECTOR_VERSION=$1
LDFLAGS="github.com/google/cloud-print-connector/lib.BuildDate=$CONNECTOR_VERSION"
CONNECTOR_DIR=$GOPATH/src/github.com/google/cloud-print-connector
MSI_FILE="$CONNECTOR_DIR/wix/windows-connector-$CONNECTOR_VERSION.msi"

echo "Running go get..."
go get -ldflags -X="$LDFLAGS" -v github.com/google/cloud-print-connector/...
rc=$?
if [[ $rc != 0 ]]; then
  echo "Error $rc with go get. Exiting."
  exit $rc
fi

echo "Running generate-dependencies.sh..."
$CONNECTOR_DIR/wix/generate-dependencies.sh
rc=$?
if [[ $rc != 0 ]]; then
  echo "Error $rc with generate-dependencies.sh. Exiting."
  exit $rc
fi

echo "Running WIX candle.exe..."
"$WIX/bin/candle.exe" -arch x64 "$CONNECTOR_DIR/wix/windows-connector.wxs" \
  "$CONNECTOR_DIR/wix/dependencies.wxs"
rc=$?
if [[ $rc != 0 ]]; then
  echo "Error $rc with WIX candle.exe. Exiting."
  exit $rc
fi

echo "Running WIX light.exe..."
"$WIX/bin/light.exe" -ext "$WIX/bin/WixUIExtension.dll" \
  "$CONNECTOR_DIR/wix/windows-connector.wixobj" "$CONNECTOR_DIR/wix/dependencies.wixobj" \
  -o "$MSI_FILE"
rc=$?
if [[ $rc != 0 ]]; then
  echo "Error $rc with WIX light.exe. Exiting."
  exit $rc
fi

echo "Successfully generated $MSI_FILE"
