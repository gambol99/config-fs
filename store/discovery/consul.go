/*
Copyright 2014 Rohith All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discovery

import (
	"errors"
	"net/url"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/golang/glog"
)

const DEFAULT_WAIT_TIME = 10

type ConsulServiceAgent struct {
	/* a lock for the watches services */
	sync.Mutex
	/* the client */
	client *consulapi.Client
	/* the wait index */
	wait_index uint64
	/* the stop channels for the services */
	watched_services map[string]chan bool
	/* the channel we use to send updates */
	update_channel ServiceUpdateChannel
}

func NewConsulServiceAgent(uri *url.URL, channel ServiceUpdateChannel) (Discovery, error) {
	glog.V(3).Infof("Creating a Consul Discovery Agent, url: %s", uri.String())
	/* step: parse the url */
	config := consulapi.DefaultConfig()
	config.Address = uri.Host
	client, err := consulapi.NewClient(config)
	if err != nil {
		glog.Errorf("Failed to create the Consul Client, error: %s", err)
		return nil, err
	}
	agent := new(ConsulServiceAgent)
	agent.update_channel = channel
	agent.watched_services = make(map[string]chan bool, 0)
	agent.client = client
	return agent, nil
}

func (r *ConsulServiceAgent) Close() error {
	r.Lock()
	defer r.Unlock()
	/* step: check if we have any watches to close */
	glog.V(VERBOSE_LEVEL).Infof("We have %d watches to shutdown resources on", len(r.watched_services))
	/* step: we iterate the watches and send a shutdown signal to end the goroutine */
	for service, channel := range r.watched_services {
		glog.V(VERBOSE_LEVEL).Infof("Closing the watch on service: %s", service)
		channel <- true
	}
	return nil
}

func (r *ConsulServiceAgent) Service(name string) (Service, error) {
	glog.V(VERBOSE_LEVEL).Infof("Service() service: %s", name)
	if services, err := r.Services(); err != nil {
		glog.Errorf("Service() failed to find services for service: %s, error: %s", name, err)
		return Service{}, err
	} else {
		for _, service := range services {
			if service.Name == name {
				return service, nil
			}
		}
	}
	return Service{}, errors.New("The service: " + name + " does not exist in discovery provider")
}

func (r *ConsulServiceAgent) Services() ([]Service, error) {
	glog.V(VERBOSE_LEVEL).Infof("Services()")
	catalog := r.client.Catalog()
	if list, _, err := catalog.Services(&consulapi.QueryOptions{}); err != nil {
		glog.Errorf("Services() failed to find services error: %s", err)
		/* step: just return an empty list */
		return make([]Service, 0), nil
	} else {
		services := make([]Service, 0)
		for name, tags := range list {
			services = append(services, Service{name, tags})
		}
		return services, nil
	}
}

func (r *ConsulServiceAgent) Endpoints(service string) ([]Endpoint, error) {
	glog.V(VERBOSE_LEVEL).Infof("Endpoints() filter: %s", service)
	catalog := r.client.Catalog()
	if services, _, err := catalog.Service(service, "", &consulapi.QueryOptions{}); err != nil {
		glog.Errorf("Endpoints() failed to find services for service: %s, error: %s", service, err)
		return nil, err
	} else {
		endpoints := make([]Endpoint, 0)
		for _, service := range services {
			endpoints = append(endpoints, r.GetEndpoint(service))
		}
		glog.V(VERBOSE_LEVEL).Infof("Endpoints() service: %s, list: %v", service, endpoints)
		return endpoints, nil
	}
}

func (r *ConsulServiceAgent) Watch(service string) error {
	r.Lock()
	defer r.Unlock()

	/* step: check if the resource is already being monitored */
	if _, found := r.watched_services[service]; found {
		glog.V(VERBOSE_LEVEL).Infof("Watch() the service %s is already being watched by this agent, skipping", service)
		return nil
	}

	glog.V(VERBOSE_LEVEL).Infof("Watch() adding a watch for changes to service: %s", service)

	/* step: we create a stop channel which is used by the goroutine below */
	shutdown_channel := make(chan bool)
	r.watched_services[service] = shutdown_channel

	go func() {
		catalog := r.client.Catalog()
		killOff := false
		r.wait_index = uint64(0)
		go func() {
			/* step: we wait for someone to send a shutdown signal */
			<-shutdown_channel
			killOff = true
		}()
		for {
			if killOff {
				glog.V(3).Infof("Watch() shutting down watch on service: %s", service)
				break
			}
			if r.wait_index == 0 {
				/* step: lets get the wait index */
				_, meta, err := catalog.Service(service, "", &consulapi.QueryOptions{})
				if err != nil {
					glog.Errorf("Watch() failed to grab the service: %s fron consul, error: %s", service, err)
					time.Sleep(5 * time.Second)
				} else {
					/* update the wait index for this service */
					r.wait_index = meta.LastIndex
				}
			}
			/* step: build the query - make sure we have a timeout */
			queryOptions := &consulapi.QueryOptions{WaitIndex: r.wait_index, WaitTime: DEFAULT_WAIT_TIME * time.Second}

			/* step: making a blocking watch call for changes on the service */
			_, meta, err := catalog.Service(service, "", queryOptions)
			if err != nil {
				glog.Errorf("Failed to wait for service to change, error: %s", err)
				r.wait_index = uint64(0)
				time.Sleep(5 * time.Second)
			} else {
				if killOff {
					continue
				}
				/* step: if the wait and last index are the same, we can continue */
				if r.wait_index == meta.LastIndex {
					continue
				}
				/* step: update the index */
				r.wait_index = meta.LastIndex
				/* step: send the update upstream */
				glog.V(VERBOSE_LEVEL).Infof("Watch() service: %s changes; sending upstream", service)
				r.update_channel <- service
			}
		}
	}()
	return nil
}

/* convert our catalog service to an endpoint */
func (r *ConsulServiceAgent) GetEndpoint(svc *consulapi.CatalogService) Endpoint {
	var endpoint Endpoint
	endpoint.ID = svc.ServiceID
	endpoint.Name = svc.ServiceName
	endpoint.Address = svc.Address
	endpoint.Port = svc.ServicePort
	return endpoint
}
