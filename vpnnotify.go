// vpnnotify - notify a user of their brand new vpn session!
// vpnnotify.go: our func main() holder
//
// Copyright 2017-2022 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.

package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/oschwald/geoip2-golang"
	"gopkg.in/redis.v5"
)

// VPNNotifyConfig - Configuration options for VPNNotify
type VPNNotifyConfig struct {
	GeoIPEnabled   bool
	GeoIPPath      string
	LDAPBaseDN     string
	LDAPPort       int
	LDAPServer     string
	LDAPSkipVerify bool
	LDAPUserAttrib string
	RedisDB        int
	RedisPassword  string
	RedisPort      int
	RedisServer    string
	RedisTLS       bool
	RenotifyTime   int64
	SlackKey       string
	TemplatePath   string
}

func main() {
	var config VPNNotifyConfig
	var configfile string

	// Get configuration information
	if os.Getenv("VPNNOTIFY_CONFIG") == "" {
		configfile = "/etc/vpnnotify.json"
	} else {
		configfile = os.Getenv("VPNNOTIFY_CONFIG")
	}
	if _, err := os.Stat(configfile); err == nil {
		config = NewConfig(configfile)
	}

	// We'll use this for the invication time
	logtime := time.Now()

	// commonName and untrustedIP are passed as environment variables from
	// OpenVPN. Exit gracefully if they dont exist, as to not cause any problems
	// if they dont exist.
	if os.Getenv("common_name") == "" {
		os.Exit(0)
	}
	if os.Getenv("untrusted_ip") == "" {
		os.Exit(0)
	}
	commonName := os.Getenv("common_name")
	untrustedIP := os.Getenv("untrusted_ip")

	// Figure out if this is in DEV or PROD. See helpers.go.
	environment := whatEnv()

	// Writing some data to Redis to capture last IP address. We'll only tell a
	// user about a login after a certain number of seconds.
	rOpts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.RedisServer, config.RedisPort),
		Password: config.RedisPassword, // no password set
		DB:       config.RedisDB,       // use DB for this (default is 0)
	}

	if config.RedisTLS {
		rOpts.TLSConfig = &tls.Config{
			ServerName: config.RedisServer,
		}
	}

	rcli := redis.NewClient(rOpts)

	// Get last IP from Redis
	lastip, err := rcli.Get(fmt.Sprintf("vpn:%s:lastip", commonName)).Result()
	if err == redis.Nil {
		lastip = ""
	} else if err != nil {
		fmt.Println(err)
	}

	// Get last login time from Redis
	lasttime, err := rcli.Get(fmt.Sprintf("vpn:%s:lasttime", commonName)).Result()
	if err == redis.Nil {
		lasttime = ""
	} else if err != nil {
		fmt.Println(err)
	}

	// Compare the last IP to the one logging in now.
	if lastip == untrustedIP {
		// time math: did it happen within the last 3900 seconds
		ltime, _ := strconv.ParseInt(lasttime, 10, 32)
		timesincelastvpn := int64(logtime.Unix()) - ltime
		if timesincelastvpn < config.RenotifyTime {
			// reset the timer, print a message to openvpn.log and skip the rest.
			updateState(rcli, logtime, commonName, untrustedIP)
			fmt.Printf("%s VPNNotify: %s login message supressed due to login in last 2 hours\n", logtime.Format("Mon Jan _2 15:04:05 2006"), commonName)
			os.Exit(0)
		}
	}
	// We're continuing, so save the last IP and time.
	updateState(rcli, logtime, commonName, untrustedIP)

	// If we're using GeoIP, get GeoIP info
	var geo *geoip2.City

	if config.GeoIPEnabled == true {
		db, err := geoip2.Open(config.GeoIPPath)
		if err != nil {
			fmt.Println(err)
		}
		geo, err = db.City(net.ParseIP(untrustedIP))
		if err != nil {
			fmt.Println(err)
		}
		db.Close()
	}

	// Build slack notification
	slackmsg := makeMessage(config, logtime, environment, geo, untrustedIP)

	// Figure out who we're sending it to
	slackuser, err := getSlackName(config, commonName)
	if err != nil {
		fmt.Printf("%s\n", err)
	}

	// Ship it
	err = sendSlack(config.SlackKey, slackuser, slackmsg)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
}
