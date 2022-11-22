# vpnnotify

`vpnnotify` is a tool that will send a user a slack message when they connect
to a VPN session. The goal of this tool is to let people know when connections
are initiated on their behalf -- with the goal being that if a connection
occurs without the user knowing, they could contact their CIRT.

`vpnnotify` builds off of the support OpenVPN has for a client connect script.
This script will run when a client successfully connects to the VPN. OpenVPN
exposes several environment variables for the script it runs to use. The ones
`vpnnotify` currently uses are `common_name` (the login name) and `untrusted_ip`
(the IP of the connection).

There are many improvements that could be made to `vpnnotify` -- as it is, we
know that this is fairly environment specific. We open sourced in hopes that we
could help level up other security operations teams and make another tool that
would contribute to the community.

# Prerequisites
In order to use `vpnnotify` you'll need to have the following:

* LDAP server with the slackUser object class
* Redis to store last logged in state
* OpenVPN - we've tested with 2.3.12 and later
* FQDNs that have either `dev` or `prod` in them as an indicator of environment

## slackUser Object Class
This adds a SlackName attribute to a record. If you use OpenLDAP with LDIF
configuration, this schema LDIF will work:

```
dn: cn=slackuser,cn=schema,cn=config
objectClass: olcSchemaConfig
cn: slackuser
olcAttributeTypes: ( 1.3.6.1.4.1.48697.1.2.1.3.1 NAME 'slackName' DESC 'Slack
  at-name' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch
  SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 SINGLE-VALUE)
olcObjectClasses: ( 1.3.6.1.4.1.48697.1.2.1.4.1 NAME 'slackUser' DESC 'Slack
  User' SUP top AUXILIARY MUST (slackName) )
```

# Building and Installing
Building `vpnnotify` is as easy as running `go build`. You could use
[FPM](https://github.com/jordansissel/fpm) to make it into a package that could
install to `/usr/bin`, if you wanted.

# Configuration
`vpnnotify` uses a configuration file which is, by default, in
`/etc/vpnnotify.json`. You can override this with the `VPNNOTIFY_CONFIG`
environment variable.

## Example Configuration
Here's a sample configuration file:

```
{
  "GeoIPEnabled": true,
  "GeoIPPath": "/opt/GeoLite2-City_20170502/GeoLite2-City.mmdb",
  "LDAPBaseDN": "dc=cool,dc=io",
  "LDAPPort": 389,
  "LDAPServer": "ldap.cool.io",
  "LDAPSkipVerify": false,
  "LDAPUserAttrib": "uid",
  "RedisDB": 0,
  "RedisPassword": "seCretsRc00l",
  "RedisPort": 6379,
  "RedisServer": "redis.cool.io",
  "RedisTLS": true,
  "RenotifyTime": 7200,
  "SlackKey": "xozz-blah-moreblah",
  "TemplatePath": "/etc/vpnnotify.msg"
}
```
## Configuration Attributes

| Variable       | Type   | Purpose                                                    | Possible Value                   |
|----------------|--------|------------------------------------------------------------|----------------------------------|
| GeoIPEnabled   | Bool   | Whether to use GeoIP. Consult their license terms.         | true, false                      |
| GeoIPPath      | String | Path to the GeoIP MMDB file                                | /opt/cooldb.mmdb                 |
| LDAPBaseDN     | String | Base DN for your LDAP server                               | dc=cool,dc=io                    |
| LDAPPort       | Int    | Port for the LDAP server (must use StartTLS)               | 389                              |
| LDAPServer     | String | FQDN for LDAP server (must use StartTLS)                   | ldap.cool.io                     |
| LDAPSkipVerify | Bool   | Whether to check the TLS certificate (for testing only!)   | false (you should keep it false) |
| LDAPUserAttrib | String | The attribute by which to look up users                    | uid                              |
| RedisDB        | Int    | The database number to use                                 | 0                                |
| RedisPassword  | String | A password for Redis                                       | secret                           |
| RedisPort      | Int    | A port for Redis                                           | 6379                             |
| RedisServer    | String | FQDN for the Redis Server                                  | redis.cool.io                    |
| RedisTLS       | Bool   | Attempt to talk to Redis using TLS                         | true, false                      |
| RenotifyTime   | Int    | Number of seconds to not renotify a user if from same IP   | 7200                             |
| SlackKey       | String | The key for Slack                                          | xozz-...                         |
| TemplatePath   | String | A path to a go template file                               | /etc/vpnnotify.msg               |

## The Template
This is where you actually put the message that will be sent via Slack. Here's a sample message template:
```
You just started a VPN session into *{{.Env}}* from {{.IP}}{{if eq .GeoIP true}} ({{if ne .City ""}}{{.City}}, {{end}}{{if ne .State ""}}{{.State}},{{end}}{{if ne .Country ""}} {{.Country}}{{end}}){{end}}. If this wasn't you, please reach out to ops immediately.
```

There aren't a lot of variables to use in the template. Here's what is available
right now:

| Variable | Type   | Purpose                                               |
|----------|--------|-------------------------------------------------------|
| Env      | String | The environment the person logged into (dev/prod/???) |
| IP       | String | The connecting IP address                             |
| GeoIP    | Bool   | If GeoIP is enabled                                   |
| City     | String | GeoIP: City                                           |
| Country  | String | GeoIP: Country                                        |
| State    | String | GeoIP: State or other municipal division              |

## Configuring OpenVPN
Set `client-connect = /path/to/vpnnotify` in your OpenVPN config. That's it.

# Testing
You can run `vpnnotify` locally. On the command line, run
`VPNNOTIFY_CONFIG=./vpnnotify.json common_name=username untrusted_ip=8.8.8.8 ./vpnnotify`
which will simulate `username` logging into the VPN from `8.8.8.8`.

When you test, you may want to set `RenotifyTime` shorter, depending on
what you're testing.

# Contributing
There are a few things we know could use some improvement:

* We currently infer environment based on the OpenVPN host's hostname - maybe
  this should be done via the configuration file? (ideal for first time
  contributors!)
* We alert on the positive condition (known user successfully connects). What
  about users who don't have the slackUser attribute? Are there other negative
  conditions we're missing?
* `vpnnotify` is fairly unsophisticated. Some statistical analysis would be
  extremely helpful in dialing down "alert fatigue" with the caveat that any
  statistical analysis should not require too much "extra" to run

## How to Contribute
Before you start contributing to any project sponsored by F5, Inc. (F5) on GitHub, you will need to sign a Contributor License Agreement (CLA). This document can be provided to you once you submit a GitHub issue that you contemplate contributing code to, or after you issue a pull request.

If you are signing as an individual, we recommend that you talk to your employer (if applicable) before signing the CLA since some employment agreements may have restrictions on your contributions to other projects. Otherwise by submitting a CLA you represent that you are legally entitled to grant the licenses recited therein.

If your employer has rights to intellectual property that you create, such as your contributions, you represent that you have received permission to make contributions on behalf of that employer, that your employer has waived such rights for your contributions, or that your employer has executed a separate CLA with F5.

If you are signing on behalf of a company, you represent that you are legally entitled to grant the license recited therein. You represent further that each employee of the entity that submits contributions is authorized to submit such contributions on behalf of the entity pursuant to the CLA.
