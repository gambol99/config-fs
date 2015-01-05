[![Build Status](https://drone.io/github.com/gambol99/config-fs/status.png)](https://drone.io/github.com/gambol99/config-fs/latest)

## Configuration File system ##

Config-fs is a key/value backed configuration file system, the daemon process synchronizes the K/V into a mount point / directory representation (presently only etcd and zookeeper are supported). Obviously changes / updates to the K/V are naturally propagated downstream.


----------

Etcd Example
----------------

	$ config-fs -store='etcd://IP:4001' -mount=/config

