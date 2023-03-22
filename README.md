dhcpd-lease-dns
===============

`dhcpd-lease-dns` is a DNS server that reads OpenBSD dhcpd `dhcpd.leases` file and
answers DNS querys on the lease entries.

On startup, it executes `tail -f dhcpd.leases`, starts the DNS server. On OpenBSD, it pledges "stdio inet proc".

Building
--------
`dhcpd-lease-dns` is a simple go program. It can simply be built with

    go build

Cross compiling for OpenBSD on any go platform:

    GOOS=openbsd go build
    
    ; Windows (untested)
    set GOOS=openbsd
    go build


Commandline
-----------

    Usage of ./dhcpd-lease-dns:
    -dhcpd-leases   string dhcpd leases file (default "/var/db/dhcpd.leases")
    -dns-port       int    DNS UDP port (default 5333)
    -domain         string local domain for replies (default "dhcp.local")

    -daemon                daemonize process
    -daemon.chroot  string chroot directory
    -daemon.pidfile string pid file name
    -daemon.syslog         log to syslog (default true)

    -quiet                 quiet normal logging
    -debug                 debug logging
    -license               show license
    -readme                show Readme.md


Configure as daemon on OpenBSD
------------------------------

/etc/rc.d/dhcpddns

    #!/bin/ksh

    daemon="/usr/local/sbin/dhcpd-lease-dns -daemon"
    daemon_user="nobody"

    . /etc/rc.d/rc.subr

    rc_reload=NO

    rc_cmd $1


/etc/rc.conf.local

    pkg_scripts=dhcpddns
    dhcpddns_flags=-domain=dhcp.my.domain


Stub-Zone in unbound
--------------------

/var/unbound/etc/unbound.conf

    server:
        local-zone: "dhcp.my.domain" transparent
        local-zone: "168.192.in-addr.arpa" transparent
        # do-not-query-localhost: no # needed if stub-addr is localhost

    stub-zone:
        name: "dhcp.my.domain."
        stub-addr: 192.168.123.123@5333

    stub-zone:                                                                                                                                                    
        name: "168.192.in-addr.arpa."                                                                                                                          
        stub-addr: 192.168.123.123@5333  
