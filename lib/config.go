/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package lib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"

	"github.com/codegangsta/cli"
)

const (
	ConnectorName = "Cloud Print Connector"

	// A website with user-friendly information.
	ConnectorHomeURL = "https://github.com/google/cups-connector"

	GCPAPIVersion = "2.0"
)

var (
	ConfigFilenameFlag = cli.StringFlag{
		Name:  "config-filename",
		Usage: fmt.Sprintf("Connector config filename (default \"%s\")", defaultConfigFilename),
		Value: defaultConfigFilename,
	}

	// To be populated by something like:
	// go install -ldflags "-X github.com/google/cups-connector/lib.BuildDate=`date +%Y.%m.%d`"
	BuildDate = "DEV"

	ShortName = platformName + " Connector " + BuildDate + "-" + runtime.GOOS

	FullName = ConnectorName + " for " + platformName + " version " + BuildDate + "-" + runtime.GOOS
)

// GetConfig reads a Config object from the config file indicated by the config
// filename flag. If no such file exists, then DefaultConfig is returned.
func GetConfig(context *cli.Context) (*Config, string, error) {
	cf, exists := getConfigFilename(context)
	if !exists {
		return &DefaultConfig, "", nil
	}

	b, err := ioutil.ReadFile(cf)
	if err != nil {
		return nil, "", err
	}

	var config Config
	if err = json.Unmarshal(b, &config); err != nil {
		return nil, "", err
	}
	return &config, cf, nil
}

// ToFile writes this Config object to the config file indicated by ConfigFile.
func (c *Config) ToFile(context *cli.Context) (string, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}

	cf, _ := getConfigFilename(context)
	if err = ioutil.WriteFile(cf, b, 0600); err != nil {
		return "", err
	}
	return cf, nil
}
