# etcdocker - etcd + docker = remote docker container discovery.

etcdocker discover remote exposed docker container.

etcdocker compatible docker client.

example

```
$ ./build
$ sudo ./bin/etcdocker run -t -d --name remote -p 20000:20 busybox /bin/sh
e4950d648c081b1d04d4ed8eefc2d324e2becbd008cb6dada9ee657e0d36b9f6
```

Link Containers

```
$ sudo docker run -t -i --link remote:busy busybox /bin/sh
/ # export
export BUSY_NAME='/focused_mclean/busy'
export BUSY_PORT='tcp://172.17.0.2:20'
export BUSY_PORT_20_TCP='tcp://172.17.0.2:20'
export BUSY_PORT_20_TCP_ADDR='172.17.0.2'
export BUSY_PORT_20_TCP_PORT='20'
export BUSY_PORT_20_TCP_PROTO='tcp'
export HOME='/'
export HOSTNAME='24db2abf1f4e'
export PATH='/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
export PWD='/'
export TERM='xterm'
/ # 
```

Use etcdocker

```
$ sudo ./bin/etcdocker run -t -i --link remote:busy busybox /bin/sh
/ # export
export BUSY_NAME='/grave_lumiere/busy'
export BUSY_PORT='tcp://192.168.122.1:20000'
export BUSY_PORT_20_TCP='tcp://192.168.122.1:20000'
export BUSY_PORT_20_TCP_ADDR='192.168.122.1'
export BUSY_PORT_20_TCP_PORT='20000'
export BUSY_PORT_20_TCP_PROTO='tcp'
export HOME='/'
export HOSTNAME='e7d2df8dbf3e'
export PATH='/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
export PWD='/'
export TERM='xterm'
/ # %
```

expose port and ipaddr discovery !
