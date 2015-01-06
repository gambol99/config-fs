[![Build Status](https://drone.io/github.com/gambol99/config-fs/status.png)](https://drone.io/github.com/gambol99/config-fs/latest)

## Configuration File system ## - [work in progress]
------

Config-fs is a key/value backed configuration file system. The daemon process synchronizes the content found in K/V store, converting the keys into files / directories and so forth. Naturally any changes which are made at the K/V end are propagated downstream to the config file system.

## Dynamic Config ##
Dynamic config works in a similar vain to [confd](https://github.com/kelseyhightower/confd). It presently supported the following methods when templating the file. Dynamic content is defined by simply prefixed the value of the K/V with "\$TEMPLATE$" (yes, not the most sophisticated means, but will work for now), note the prefix is removed from the actual content.

**Service Method:**
**Example:** service "frontend_http"

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

**GetValue Method:**
**Example:** getv "/this/is/a/key/im/interested/in"

The GetValue() method is used to retrieve and watch a key with in the K/V store.

Usage:

    username: product_user
    password: {{ getv "/prod/config/db/password"

**GetList Method:**
**Example:** getr "/this/is/a/key/im/interested/"

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


