[![Build Status](https://drone.io/github.com/gambol99/config-fs/status.png)](https://drone.io/github.com/gambol99/config-fs/latest)

## Configuration File system ## - [work in progress]
------

Config-fs is a key/value backed configuration file system. The daemon process synchronizes the content found in K/V store, converting the keys into files / directories and so forth. Naturally any changes which are made at the K/V end are propagated downstream to the config file system.

Current Status
------

 - Replication of the K/V store to the configuration directory is working
 - The watcher service needs to be completed and integrated - thus allowing for write access to the backend (though not a priority at the moment)
 - The dynamic resources work, but requires a code clean up, a review and no doubt a number of bug fixes

Configuration
------

       [jest@starfury config-fs]$ stage/config-fs --help
       Usage of stage/config-fs:
         -alsologtostderr=false: log to standard error as well as files
         -delete_on_exit=false: delete all configuration on exit
         -delete_stale=false: delete stale files, i.e files which do not exists in the backend k/v store
         -discovery="": the service discovery backend being used
         -interval=900: the default interval for performed a forced resync
         -log_backtrace_at=:0: when logging hits line file:N, emit a stack trace
         -log_dir="": If non-empty, write log files in this directory
         -logtostderr=false: log to standard error instead of files
         -mount="/config": the mount point for the K/V store
         -pre_sync=true: wheather or not to perform a initial config sync against the backend
         -read_only=true: wheather or not the config store of read-only
         -root="/": the root within the k/v store to base the config on
         -stderrthreshold=0: logs at or above this threshold go to stderr
         -store="etcd://localhost:4001": the url for key / value store
         -v=0: log level for V logs
         -vmodule=: comma-separated list of pattern=N settings for file-filtered logging

Configuration Root
-----

By default the configuration directory is build from root "/", the -root=KEY can override this though. A use case for this would be hide expose only a subsection of the k/v store. For example, we can expose /prod/app/config directory to /config while hiding everything underneath; note: ALL dynamic configs take keys from root "/", so in our case we expose the config files, which placing the credentials, values, config etc which the dynamic config reference hidden beneath.

## Dynamic Config ##
Dynamic config works in a similar vain to [confd](https://github.com/kelseyhightower/confd). It presently supported the following methods when templating the file. Dynamic content is defined by simply prefixed the value of the K/V with "\$TEMPLATE$" (yes, not the most sophisticated means, but will work for now), note the prefix is removed from the actual content.

- **Service Method**:
- **Example**: {{ service "frontend_http" }}

The method is used to perform a lookup on a service discovery provider (at present I'm using "consul"). The function will respond with an array of endpoints; Note, every time a service "name' is used within a template a watch is automatically placed on the service, with any changes to the endpoints throwing a notification to the store and regenerating the content

    type Endpoint struct {
      ID string
      /* the name of the service */
      Name string
      /* the ip address of the service */
      Address string
      /* the port the service is running on */
      Port int
    }

Usage:

    {{range service .Value}}  server {{.Address}}_{{.Port}} {{.Address}}:{{.Port}}{{printf "\n"}}{{end}}
    # Producing something like;
    server 10.241.1.75_31002 10.241.1.75:31002

- **Endpoints Method**:
- **Example**: {{ endpoints "frontend_http" }}

Similar to the above method, it returns an array of <IPADDRESS>:<PORT> endpoints in the service request. 

Usage:

	{{$services := getvs "/services/elasticsearch/*"}}
	services: {{join $services ","}}

- **GetValue Method:**
- **Example:** getv "/this/is/a/key/im/interested/in"

The GetValue() method is used to retrieve and watch a key with in the K/V store.

Usage:

    username: product_user
    password: {{ getv "/prod/config/db/password"

-  **GetList Method:**
- **Example:** getr "/this/is/a/key/im/interested/"

The GetList() method is used to produce an aray of child keys (excluding directories) under the path specified.

    type Entry struct {
      /* the path of the node */
      Path string
      /* the value of the node */
      Value string
    }

Usage:

Lets just assume for some incredible reason we are using the directory /prod/config/zookeeper to keep a list of zookeepers ... i.e. /prod/config/zookeeper/zoo101 => 10.241.1.100, /prod/config/zookeeper/zoo102 => 10.241.1.101 etc.

    {{range getr "/prod/config/zookeeper"}}
    server {{.Value}}{{end}

**UnmarshallJSON Method:**
**Example:** getv "some_key | json"

The json(), a pipeline method, taking the argument previous method and unmarshalling the value.


