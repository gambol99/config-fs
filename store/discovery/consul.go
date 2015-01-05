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
	"time"

	consulapi "github.com/armon/consul-api"
	"github.com/golang/glog"
)

const DEFAULT_WAIT_TIME = 180

type ConsulServiceAgent struct {
	/* the client */
	client *consulapi.Client
	/* the wait index */
	waitIndex uint64
	/* the shutdown signal */
	shutdownChannel chan bool
}

func (r *ConsulServiceAgent) Close() error {
	glog.Infof("Closing the consul agent")
	r.shutdownChannel <- true
	return nil
}

func (r *ConsulServiceAgent) ListEndpoints(service string) ([]Endpoint, error) {
	glog.V(VERBOSE_LEVEL).Infof("ListEndpoints() filter: %s", service)
	catalog := r.client.Catalog()
	if services, _, err := catalog.Service(service, "", &consulapi.QueryOptions{}); err != nil {
		glog.Errorf("FindService() failed to find services for service: %s, error: %s", service, err)
		return nil, err
	} else {
		endpoints := make([]Endpoint, 0)
		for _, service := range services {
			endpoints = append(endpoints, r.GetService(service))
		}
		return endpoints, nil
	}
}

func (r *ConsulServiceAgent) WatchService(service string, updateChannel ServiceUpdateChannel) (chan bool, error) {
	glog.V(VERBOSE_LEVEL).Infof("WatchServices() watching for changes to service: %s, channel: %v", service, updateChannel)
	shutdownChannel := make(chan bool)
	go func() {
		catalog := r.client.Catalog()
		killOff := false
		r.waitIndex = uint64(0)
		/* step wait for a shutdown signal */
		go func() {
			<-shutdownChannel
			killOff = true
		}()
		for {
			if killOff {
				glog.V(3).Infof("WatchServices() shutting down watch on service: %s", service)
				break
			}
			if r.waitIndex == 0 {
				/* step: lets get the wait index */
				_, meta, err := catalog.Service(service, "", &consulapi.QueryOptions{})
				if err != nil {
					glog.Errorf("WatchServices() failed to grab the service: %s fron consul, error: %s", service, err)
					time.Sleep(5 * time.Second)
				} else {
					/* update the wait index for this service */
					r.waitIndex = meta.LastIndex
				}
			}
			/* step: build the query - make sure we have a timeout */
			queryOptions := &consulapi.QueryOptions{WaitIndex: r.waitIndex, WaitTime: DEFAULT_WAIT_TIME}

			/* step: making a blocking watch call for changes on the service */
			_, meta, err := catalog.Service(service, "", queryOptions)
			if err != nil {
				glog.Errorf("Failed to wait for service to change, error: %s", err)
				r.waitIndex = uint64(0)
				time.Sleep(5 * time.Second)
			} else {
				if killOff {
					continue
				}
				/* step: if the wait and last index are the same, we can continue */
				if r.waitIndex == meta.LastIndex {
					glog.V(VERBOSE_LEVEL).Infof("The WaitIndex and LastIndex are the same, skipping")
					continue
				}
				/* step: update the index */
				r.waitIndex = meta.LastIndex
				/* step: send the update upstream */
				glog.V(VERBOSE_LEVEL).Infof("WatchServices() service: %s changes; sending upstream", service )
				updateChannel <- service
			}
		}
	}()
	return shutdownChannel, nil
}

func (r *ConsulServiceAgent) GetService(svc *consulapi.CatalogService) (Endpoint) {
	var endpoint Endpoint
	endpoint.ID = svc.ServiceID
	endpoint.Name = svc.ServiceName
	endpoint.Address = svc.Address
	endpoint.Port = svc.ServicePort
	return endpoint
}
