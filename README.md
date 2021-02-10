# sacloudns
simple command line tool for sakura cloud dns

## Download and Install

For Mac, use homebrew tap

```
$ brew install kazeburo/tap/sacloudns
```

### Download from GitHub Releases

It's able to download from GitHub Release.

https://github.com/kazeburo/sacloudns/releases

## Usage

```
Usage:
  sacloudns [OPTIONS] <command>

Help Options:
  -h, --help  Show this help message

Available commands:
  list     list zones
  radd     add a record
  rdelete  delete a record
  rset     replace records or add a record
  zone     describe zone
```

`SAKURACLOUD_ACCESS_TOKEN` and `SAKURACLOUD_ACCESS_TOKEN_SECRET` environment values are required for API request.

## list zone

```
$ sacloudns list
```

## fetch zone

```
Usage:
  sacloudns [OPTIONS] zone [zone-OPTIONS]

Help Options:
  -h, --help      Show this help message

[zone command options]
          --name= dnszone name to find
```

```
$ sacloudns zone example.com
```

## Add a record

```
Usage:
  sacloudns [OPTIONS] radd [radd-OPTIONS]

Help Options:
  -h, --help              Show this help message

[radd command options]
          --zone=         dnszone name to add a record
          --ttl=          record TTL to add (default: 300)
          --name=         record NAME to add
          --type=         record TYPE to add
          --data=         record DATA to add
          --wait          wait for record propagation
          --wait-timeout= wait timeout for record propagation (default: 60s)
```

Add an A record

```
./sacloudns radd --zone example.com --name www --type A --data 192.168.0.1 --ttl 30
```

Add a TXT for DNS challenge

```
./sacloudns radd --wait --zone example.com --name _acme --type TXT --data xxxxxx --ttl 30
```

with `--wait` option, sacloudns wait for DNS record propergation. `--wait` is only available for TXT and CNAME

## Set a record

```
Usage:
  main [OPTIONS] rset [rset-OPTIONS]

Help Options:
  -h, --help              Show this help message

[rset command options]
          --zone=         dnszone name to set a record
          --ttl=          record TTL to set (default: 300)
          --name=         record NAME to set
          --type=         record TYPE to set
          --data=         record DATA to set
          --wait          wait for record propagation
          --wait-timeout= wait timeout for record propagation (default: 60s)
```

As same as radd, with `--wait` option, sacloudns wait for DNS record propergation. `--wait` is only available for TXT and CNAME

## Delete a record

```
Usage:
  sacloudns [OPTIONS] rdelete [rdelete-OPTIONS]

Help Options:
  -h, --help      Show this help message

[rdelete command options]
          --zone= dnszone name to set a record
          --name= record NAME to set
          --type= record TYPE to set
          --data= record DATA to set
```

```
./sacloudns rdelete --zone example.com --name test --type A --data 192.168.0.1
```

