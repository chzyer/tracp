# Tracp

TRAffic Control in Port level
platform: OSX/Linux

Tracp will handle all packets which gothrough it and redirect them back to lookback.


# Usage:

```
usage: tracpd [option]

options:
    -droprate <number>  0-100
    -bandwidth <number> max bandwidth per second, unit: byte
    -mindelay           min delay time
    -maxdelay           max delay time
    -h                  show help
```

# example

```
# go install github.com/chzyer/tracp/cmd/tracpd
$ sudo tracpd -mindelay 200ms
$ ping 10.1 # use 10.0.0.1 instead of 127.0.0.1
PING 10.1 (10.0.0.1): 56 data bytes
64 bytes from 10.0.0.1: icmp_seq=0 ttl=64 time=420.205 ms
64 bytes from 10.0.0.1: icmp_seq=1 ttl=64 time=417.257 ms
64 bytes from 10.0.0.1: icmp_seq=2 ttl=64 time=413.291 ms
64 bytes from 10.0.0.1: icmp_seq=3 ttl=64 time=400.472 ms
```
