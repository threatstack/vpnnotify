// vpnnotify - notify a user of their brand new vpn session!
// helpers.go: a collection of functions
//
// Copyright 2017 Threat Stack, Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"gopkg.in/ldap.v2"
	"gopkg.in/redis.v5"
)

// whatEnv - determines if you're VPNing into DEV or PROD depending on the
// fqdn of the host. This is, admittedly, not great - and it is frustrating
// that Golang doesnt have some sort of way of getting the output of hostname -f
// "natively" so this will have to do.
func whatEnv() string {
	cmd := exec.Command("/bin/hostname", "-f")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	}
	fqdn := out.String()
	fqdn = fqdn[:len(fqdn)-1] // removing EOL

	env := ""
	if strings.Contains(fqdn, "dev") {
		env = "dev"
	} else if strings.Contains(fqdn, "prod") {
		env = "prod"
	} else if strings.Contains(fqdn, "bunker") {
		env = "bunker"
	} else if strings.Contains(fqdn, "ua") {
		env = "ua"
	} else {
		env = "NONE"
	}
	return env
}

// NewConfig - read in and parse a JSON configuration file into the
// VPNNotifyConfig struct located in types.go.
func NewConfig(fname string) VPNNotifyConfig {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	config := VPNNotifyConfig{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}
	return config
}

// updateState - updates the Redis database with the last time we saw an IP
// connect for a particular user.
func updateState(rcli *redis.Client, lt time.Time, commonName string, untrustedIP string) {
	err := rcli.Set(fmt.Sprintf("vpn:%s:lastip", commonName), untrustedIP, 0).Err()
	if err != nil {
		fmt.Printf("%s VPNNotify: %s couldnt save to redis: %s \n", lt.Format("Mon Jan _2 15:04:05 2006"), commonName, err)
	}
	err = rcli.Set(fmt.Sprintf("vpn:%s:lasttime", commonName), int64(lt.Unix()), 0).Err()
	if err != nil {
		fmt.Printf("%s VPNNotify: %s couldnt save to redis: %s \n", lt.Format("Mon Jan _2 15:04:05 2006"), commonName, err)
	}
}

// sendSlack - send a slack message to a user informing them of a VPN login.
func sendSlack(key string, recipient string, message string) (err error) {
	api := slack.New(key)
	opts := slack.MsgOptionCompose(
		slack.MsgOptionAsUser(true),
		slack.MsgOptionDisableLinkUnfurl(),
		slack.MsgOptionText(message, false),
	)

	// Fire message
	channelID, timestamp, err := api.PostMessage(recipient, opts)
	if err != nil {
		return err
	}

	timestamp64, err := strconv.ParseFloat(timestamp, 64)
	if err != nil {
		return err
	}
	tm := time.Unix(int64(timestamp64), 0).Format("Mon Jan _2 15:04:05 2006")
	// This'll go into openvpn.log.
	fmt.Printf("%s VPNNotify: message sent to %s (%s)\n", tm, channelID, recipient)
	return nil
}

// getSlackName - Get a user's Slack name from LDAP. This requires a new LDAP
// schema entry -- see documentation.
func getSlackName(config VPNNotifyConfig, commonName string) (name string, err error) {
	conntimeout := time.Duration(5) * time.Second
	server, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", config.LDAPServer, config.LDAPPort), conntimeout)
	if err != nil {
		return "", err
	}

	l := ldap.NewConn(server, false)
	l.Start()
	defer l.Close()

	// Need a place to store TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.LDAPSkipVerify,
		ServerName:         config.LDAPServer,
	}

	// TLS our connection up
	err = l.StartTLS(tlsConfig)
	if err != nil {
		return "", err
	}

	// Set up an LDAP search and actually do the search
	searchRequest := ldap.NewSearchRequest(
		config.LDAPBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(%s=%s)", config.LDAPUserAttrib, commonName),
		[]string{"slackName"},
		nil,
	)
	sr, err := l.Search(searchRequest)
	if err != nil {
		return "", err
	}

	if len(sr.Entries) == 0 {
		return "", errors.New("ENOENTRIES")
	} else if len(sr.Entries) > 1 {
		return "", errors.New("ETOOMANYENTRIES")
	}

	return fmt.Sprintf("@%s", sr.Entries[0].GetAttributeValue("slackName")), nil
}
