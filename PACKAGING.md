# Building system packages

To build system packages for Linux (`deb`, `rpm`, etc), use the included
`scripts/build.py` script.

```sh
$ # Get all the code and dependencies into a temporary $GOPATH
$ export GOPATH=/tmp/go
$ go get -d -u github.com/vente-privee/influxdb-relay
$ cd $GOPATH/src/github.com/vente-privee/influxdb-relay
$ scripts/build.py
```

The packages will be found under
`$GOPATH/src/github.com/vente-privee/influxdb-relay/build`

For more build options, check the build script arguments:
```sh
$ scripts/build.py -h
```

For example, to build versioned packages use:
```sh
$ scripts/build.py --package --version 2.3.0
```

## Dependencies

1. Go 1.5+ is required. Use https://go-repo.io/ to install on CentOS/Fedora.
2. FPM Ruby gem is needed - use
https://fpm.readthedocs.io/en/latest/installing.html as an installation guide
for Fedora/CentOS and Debian/Ubuntu.
