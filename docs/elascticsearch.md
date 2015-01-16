
## **Elasticsearch Cluster Demo**

> The following details a usage example for building an elasticsearch cluster

#### **Requirements**

 - A working CoreOS cluster - if you've not got this in place, take a look at ([coreos-example](https://github.com/gambol99/coreos-vagrant)
 - Service registration for container services[Consul](www.consul.io) in place. Again, take a look at the above link if you've not got anything in place

#### **Config-FS (config-fs.service)**

Below is the unit file for the config-fs service

    [Unit]
    Description=Configuration Store
    After=docker.service
    Requires=docker.service

    [Service]
    EnvironmentFile=/etc/environment
    #Restart=on-failure
    #RestartSec=5
    TimeoutStartSec=0
    ExecStartPre=-/usr/bin/docker kill config-store
    ExecStartPre=-/usr/bin/docker rm config-store
    ExecStartPre=/usr/bin/docker pull gambol99/config-fs
    ExecStart=/usr/bin/sh -c "/usr/bin/docker run \
      --name config-store \
      -e NAME=config-store \
      -e ENVIRONMENT=prod \
      -e CONFIG_URL=etcd://${COREOS_PRIVATE_IPV4}:4001 \
      -e DISCOVERY_URL=consul://${COREOS_PRIVATE_IPV4}:8500 \
      -e VERBOSITY=3 \
      -v /config:/config \
      gambol99/config-fs"

    ExecStop=/usr/bin/docker stop config-store
    ExecStop=/usr/bin/docker rm config-store

    [Install]
    WantedBy=multi-user.target

    [X-Fleet]
    Global=true


> **Make sure the service is running**: with $ fleetctl list-units

#### **Elasticsearch Dockerfile**

The full code can be found at [elasticsearch](https://github.com/gambol99/coreos-vagrant/tree/master/dockers/elasticsearch)

    FROM centos:latest
    MAINTAINER Rohith <gambol99@gmail.com>

    ADD config/elasticsearch.repo /etc/yum.repos.d/elasticsearch.repo
    ADD config/elasticsearch.yml /usr/share/elasticsearch/config/elasticsearch.yml
    ADD config/startup.sh /startup.sh

    RUN rpm --import https://packages.elasticsearch.org/GPG-KEY-elasticsearch
    RUN yum install -y java-1.7.0-openjdk.x86_64 elasticsearch

    VOLUME [ "/data" ]
    EXPOSE 9200 9300
    WORKDIR /data
    CMD [ "/startup.sh" ]

The startup script for elasticsearch - *note, it's not supposed to be extensive, just workable*

    #!/bin/bash
    annonce() { echo "* $@"; }
    VERSION="1.4.1"
    NAME="Elasticsearch ${VERSION}"
    SERVICE_DELAY=${SERVICE_DELAY:-5}
    JAVA_OPTS=${JAVA_OPTS:-""}
    JAVA_OPTS="-Des.transport.publish_host=${HOST} \
      -Des.transport.publish_port=${PORT_9300}"

    [ -n "${CLUSTER}" ] && JAVA_OPTS="${JAVA_OPTS} -Des.cluster.name=${CLUSTER}"
    [ -n "${NODE_NAME}" ] && JAVA_OPTS="${JAVA_OPTS} -Des.node.name=${NODE_NAME}"

    annonce "Waiting for ${SERVICE_DELAY} seconds before starting up ${NAME}"
    sleep ${SERVICE_DELAY}

    annonce "Starting ${NAME} service:

    /usr/share/elasticsearch/bin/elasticsearch ${JAVA_OPTS}

#### **CoreOS - Unit file**

FIlename: elasticsearch@.service

    [Unit]
    Description=ElasticSearch Node %i
    After=etcd.service
    After=docker.service
    Requires=etcd.service
    Requires=docker.service

    [Service]
    EnvironmentFile=/etc/environment
    TimeoutStartSec=0
    Restart=on-failure
    ExecStartPre=-/usr/bin/docker kill elasticsearch.%s.node
    ExecStartPre=-/usr/bin/docker rm elasticsearch.%s.node
    ExecStartPre=/usr/bin/docker pull quay.io/gambol99/elasticsearch
    ExecStart=/usr/bin/docker run -P \
    --name elasticsearch.%s.node \
    -e ENVIRONMENT=prod \
    -e NAME=elasticsearch \
    -e CLUSTER=es_cluster \
    -e PLUGINS=mobz/elasticsearch-head \
    -e SERVICE_9200_NAME=es_prod_client \
    -e SERVICE_9300_NAME=es_prod_peer \
    -e VERBOSITY=3 \
    -v /config/prod/cfg/es_cluster/es.yml:/usr/share/elasticsearch/config/elasticsearch.yml \
    quay.io/gambol99/elasticsearch

    ExecStop=/usr/bin/docker stop embassy.proxy

    [Install]
    WantedBy=multi-user.target

Lets create a few instances
 
    $ for i in {1..4}; do
    >   ln -s elasticsearch@.service elasticsearch@${i}.service
    > done

#### **Create the dynamic config**

The elasticsearch dynamic config template

    $TEMPLATE$
    cluster.name: es-prod
    node.master: true
    node.data: true
    index.number_of_shards: 5
    index.number_of_replicas: 1
    path.conf: /etc/elasticsearch
    path.data: /data
    #bootstrap.mlockall: true
    discovery.zen.minimum_master_nodes: 1
    discovery.zen.ping.timeout: 3s
    {{ $endpoints := endpointsl "es_prod_peer" }}
    discovery.zen.ping.unicast.hosts: [ "{{ join $endpoints "\",\"" }}" ]

Add the template to the K/V store
 
    $ # copy the above template into a filename: es.yml
    $ DOC=$(cat es.yaml)
    $ etcdctl set /prod/cfg/es_cluster/es.yml "$DOC"
    # make sure the template has been created
    $ ls /config/prod/cfg/es_cluster/es.yml
    $ cat $!

#### **Launch elasticsearch**
 
    $ fleetctl start elasticsearch@1.service
    # .. and wait for the service to pop up.
    $ watch fleetctl list-units
    # validate the service has populated in the dynamic config
    $ cat /config/prod/cfg/es_cluster/es.yml
    # is not, make sure the service discovery is funtioning, that consul has registered the service?
  
    # we can now launch the rest of them
    $ fleetctl start  elasticsearch@[234].service
    $ watch fleetclt list-units -full

#### **Validate the cluster**
 
    # Take a elasticseach instance - grab the 9200 port and
    $ curl http://<IP>:<PORT>/_cluster/health?pretty=true
