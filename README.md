[![Build Status](https://drone.io/github.com/gambol99/config-fs/status.png)](https://drone.io/github.com/gambol99/config-fs/latest)

## Configuration File system ##
------

Config-fs is a key/value backed configuration file system. The daemon process synchronizes the content found in K/V store, converting the keys into files / directories and so forth. Naturally any changes which are made at the K/V end are propagated downstream to the config file system.

#### **Current Status**

 - Replication of the K/V store to the configuration directory is working
 - The watcher service needs to be completed and integrated - thus allowing for write access to the backend (though not a priority at the moment)
 - The dynamic resources work, but requires a code clean up, a review and no doubt a number of bug fixes
 - Presently only supports etcd for the K/V store and consul for discovery 

#### **Configuration**

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

#### **Configuration Root**

By default the configuration directory is build from root "/", the -root=KEY can override this though. A use case for this would be hide expose only a subsection of the k/v store. For example, we can expose /prod/app/config directory to /config while hiding everything underneath; note: ALL dynamic configs take keys from root "/", so in our case we expose the config files, which placing the credentials, values, config etc which the dynamic config reference hidden beneath.

#### **Docker Usage**

	#/usr/bin/docker run \
	  --name config-store \
	  -e NAME=config-store \
	  -e ENVIRONMENT=prod \
	  -e PRIVATE_IP=${COREOS_PRIVATE_IPV4} \
	  -e VERBOSITY=10 \
	  -v /config:/config \
	  gambol99/config-fs \
	  -mount=/config \
	  -store=etcd://${COREOS_PRIVATE_IPV4}:4001 \
	  -discovery=consul://${COREOS_PRIVATE_IPV4}:8500 \
	  -root=/env/prod

#### **Dynamic Config** 

Dynamic config works in a similar vain to [confd](https://github.com/kelseyhightower/confd). It presently supported the following methods when templating the file. Dynamic content is defined by simply prefixed the value of the K/V with "$TEMPLATE$" (yes, not the most sophisticated means, but will work for now), note the prefix is removed from the actual content.

##### {{ service "frontend_http" }}

The method search the discovery provider for a service, returning the following struct

    type Service struct {
    	/* the descriptive for the service */
    	Name string
    	/* the tags associated to the service */
    	Tags []string
    }

    {{ $service := service "frontend_http" }}
    {{ $service.Tags }}

##### {{ services }}

Returns a array of all services from the discovery provider

    {{ range services }}
    Name: {{ .Name }}, Tags: {{ print "," | join .Tags }}
    {{ end }}

##### {{ endpoints "frontend_http" }}

Similar to the above method, it returns an array of <IPADDRESS>:<PORT> endpoints in the service request.

    The struct returned

    type Endpoint struct {
      ID string
      /* the name of the service */
      Name string
      /* the ip address of the service */
      Address string
      /* the port the service is running on */
      Port int
    }

    {{ range getl "/services/frontends/" }}
    frontend {{ base .Path }}
      bind *: {{ getv "/services/frontend/port" }}
      mode http
      default_backend {{ base .Path }}

    backend {{ base .Path }}
      mode http
      balance roundrobin
      option forwardfor
      http-request set-header X-Forwarded-Port %[dst_port]
      http-request add-header X-Forwarded-Proto https if { ssl_fc }
    {{ range service .Path }}  server {{.Address}}_{{.Port}} {{.Address}}:{{.Port}}{{printf "\n"}}{{end}}
    {{ end }}

    or

    {{range endpoints "frontend_http"}}  server {{.Address}}_{{.Port}} {{.Address}}:{{.Port}}{{printf "\n"}}{{end}}
    # Producing something like
    server 10.241.1.75_31002 10.241.1.75:31002
    server 10.241.1.75_31001 10.241.1.75:31001

##### {{ endpointsl "frontend_http" }}

A helper method for the one above, it simply hands back the endpoints as a array of strings

	{{$services := endpoints "frontend_http"}}
	services: {{join $services ","}}

##### {{ get "/this/is/a/key/im/interested/in" }}

The Get() method is used to retrieve and watch a key/pair with in the K/V store.

    type Node struct {
        /* the path for this key */
        Path string
        /* the value of the key */
        Value string
        /* the type of node it is, directory or file */
        Directory bool
    }

    {{$node := get "/prod/config/database/password" }}
    {{$node.Path}} {{$node.Value}}

##### {{ gets "/this/is/a/key/im/interested/in" }}

Returns an array of keypairs - essentially a list of children (files/directories) under the path

    {{ range gets "/services/http/frontends" }}
    {{ service

    {{ end }}

##### {{ getv "/this/is/a/key/im/interested/in" }}

The GetValue() method is used to retrieve 'value' of a key with in the K/V store.

    username: product_user
    password: {{ getv "/prod/config/db/password" }}

##### {{ getr "/this/is/a/key/im/interested/" }}

The GetList() method is used to produce an aray of child keys (excluding directories) under the path specified.

Lets just assume for some incredible reason we are using the directory /prod/config/zookeeper to keep a list of zookeepers ... i.e. /prod/config/zookeeper/zoo101 => 10.241.1.100, /prod/config/zookeeper/zoo102 => 10.241.1.101 etc.

    {{range getr "/prod/config/zookeeper"}}
    server {{.Value}}{{end}

##### contained

Checks to see if a value is inside an array i.e. check if a tag is present in a service

    {{ if "haproxy" | contained .Tags }}
    Not really a fan of this pipeline thing ... give me ERB any day
    {{end}}

##### json

Takes a argument, hopefully some json string and unmarshalls string in a map[string]value

##### jsona

Takes a json string and unmarshalls string in an array of map[string]value

##### Additional (well add example later)

    "base":      path.Base,
    "dir":       path.Dir,
    "split":     strings.Split,
    "getenv":    os.Getenv,
    "join":      strings.Join}
------

#### **Contributing**

 - Fork it
 - Create your feature branch (git checkout -b my-new-feature)
 - Commit your changes (git commit -am 'Add some feature')
 - Push to the branch (git push origin my-new-feature)
 - Create new Pull Request
 - If applicable, update the README.md
