// vpnnotify - notify a user of their brand new vpn session!
// template.go: code related to building a notification
//
// Copyright 2017 Threat Stack, Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package main

import (
	"bytes"
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"io/ioutil"
	"text/template"
	"time"
)

// templInfo is the info we can pass to template.New
type templInfo struct {
	City    string
	Country string
	Env     string
	GeoIP   bool
	IP      string
	State   string
}

// makeMessage: This is where we actually build the message to go to the user
func makeMessage(config VPNNotifyConfig, lt time.Time, env string, geo *geoip2.City, ip string) string {
	rawTemplate, err := ioutil.ReadFile(config.TemplatePath)
	if err != nil {
		fmt.Printf("%s VPNNotify: Unable to read template for Slack message\n", lt.Format("Mon Jan _2 15:04:05 2006"))
	}
	tmpl := template.Must(template.New("vpnnotify").Parse(string(rawTemplate)))

	var vars templInfo
	vars.IP = ip
	vars.Env = env

	if config.GeoIPEnabled {
		vars.GeoIP = true
		vars.City = geo.City.Names["en"]
		vars.State = geo.Subdivisions[0].Names["en"]
		vars.Country = geo.Country.IsoCode
	} else {
		vars.GeoIP = false
	}

	var rendered bytes.Buffer
	err = tmpl.Execute(&rendered, vars)
	if err != nil {
		fmt.Printf("%s VPNNotify: Unable to render template for Slack message\n", lt.Format("Mon Jan _2 15:04:05 2006"))
	}
	return rendered.String()
}
