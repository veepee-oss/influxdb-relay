# v 0.6.1 (2020-05-10)

## New features

* Added new building system for binary,rpm/dev packages and also docker image
* Dockerfile now build image from local sources not from remote repo
* Added new -pidfile and -logs options ( -logdir still running but will be removed in the future )

## fixes

* Fix #28,#22

# v 0.6.0 (2019-06-27)

## fixes

* Fix #19,#18

# v 0.5.0 (2019-06-13)

## New features

* added TLS paramters tls_cer, tls_key for HTTP endpoint

## fixes

* Fix #7,#16,#3

## Breaking changes

* removed old ssl-combined-pem, changed by tls_cer, tls_key

# v 0.4.0 (2019-05-29)

## New features
* added kb_duratiom_ms and latency_ms fields to the access.log
* Added Online Config check and reload if ok, Kill -HUP
* improved doc in sample.conf

## fixes

* Fix #11,#10
* fixed distribution packages descriptions

# v 0.3.0 (2019-05-23)

## New features

* added `/admin/<cluster_id>` API for clusters
* added HA example

# v 0.2.0 (2019-05-16)

## New features

* improved ping,health,status http responses
* added config validations
* Second big refactor.
* Improved docker image

## fixes

* fix HTTP response thus if not matched with any route
* fix /health  handler for the process rather than the cluster
* fix log path when relative, will be now relative to logdir if configured and with CWD if not

# v 0.1.0 (2019-04-25)

## New features

* renamed project from influxdb-relay to influxdb-srelay 
* first big refactor
* added influx IQL query support, and query-router-endpoint-api parameter to get a list of available influx ID's to send /query quieries

# PREVIOUS RELEASES


Jun 28 2018 Antoine MILLET <amillet@vente-privee.com>
	From https://github.com/influxdata/influxdb-srelay

Jun 29 2018 Alexandre BESLIC <abeslic@abronan.com>
	* Switch to dep, make the project buildable after go get

Jun 29 2018 Dejan FILIPOVIC <dfilipovic@vente-privee.com>
	* Add /status route

Jun 30 2018 Antoine MILLET <amillet@vente-privee.com>
	* New README structure
	* Add basic tests with golint & pylint
	* Add CHANGELOG
	* Add CONTRIBUTING guide
	* Merge https://github.com/influxdata/influxdb-relay/pull/65
	* Merge https://github.com/influxdata/influxdb-relay/pull/52
	* Merge https://github.com/influxdata/influxdb-relay/pull/59
	* Merge https://github.com/influxdata/influxdb-relay/pull/43
	* Merge https://github.com/influxdata/influxdb-relay/pull/57

Nov 15 2018 Maxime CORBIN <mcorbin@vente-privee.com>
    * Add Prometheus input support
    * Add `/admin` route to administrate underlying databases
    * Add code coverage / unit tests
    * Code refactor
    * Improve buffering feature avoiding connexions hanging
    * Improve `/ping` route
    * Improve logging
    * Add `-version` option
    
Dec 13 2018 Maxime CORBIN <mcorbin@vente-privee.com>
    * Add `/admin` endpoint that can be used to create or remove databases
    * Add `/health` endpoint that can be used to monitor the health status of every backend
    * Fixed some performance bugs & added a few more logs
    
Dec 27 2018 Clément CORNUT <ccornut@vente-privee.com>
    * Add `/admin/flush` endpoint that can be used to flush internal buffer
    * Add Rate limiting on backend
    * Add Filters for tags and measurements
    * New configuration formatting
    * Various bug fixes
Apr 8 2019 Toni Moreno <toni.moreno@gmail.com>
    * added influx IQL query support, and query-router-endpoint-api parameter to get a list of available influx ID's to send /query quieries"
Apr 25 2019 Toni Moreno <toni.moreno@gmail.com>
    * From https://github.com/vente-privee/influxdb-relay
    * Renamed to influxdb-srelay ( Smart Relay) 
    * Refactor to new config type

	
