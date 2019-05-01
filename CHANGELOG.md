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
	* Merge https://github.com/influxdata/influxdb-srelay/pull/65
	* Merge https://github.com/influxdata/influxdb-srelay/pull/52
	* Merge https://github.com/influxdata/influxdb-srelay/pull/59
	* Merge https://github.com/influxdata/influxdb-srelay/pull/43
	* Merge https://github.com/influxdata/influxdb-srelay/pull/57

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
    * From https://github.com/vente-privee/influxdb-srelay
    * Renamed to influxdb-srelay ( Smart Relay) 
    * Refactor to new config type

	
