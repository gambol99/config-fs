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

package agent

import (
    "time"

    consulapi "github.com/armon/consul-api"
    "github.com/golang/glog"
)

const (
    defaultWaitTime = 120
)

type ConsulServiceAgent struct {
    /* the client */
    Client  *consulapi.Client
    /* the wait index */
    WaitIndex uint64
}

func (r *ConsulServiceAgent) FindServices(filter string) ([]Service, error) {
    Verbose("FindServices() filter: %s", filter )
    catalog := r.Client.Catalog()
    services, _, err := catalog.Service( filter, "", &consulapi.QueryOptions{} )
    if err != nil {
        glog.Errorf("FindService() failed to find services for service: %s, error: %s", filter, err )
        return nil, err
    }
    endpoints := make([]Service,0)
    for _, service := range services {
        endpoints = append(endpoints, r.GetService( service ) )
    }
    return endpoints, nil
}

func (r *ConsulServiceAgent) WatchServices(service *Service, updateChannel chan *Service) (chan bool, error) {
    Verbose("WatchServices() watching for changes to service: %s, channel: %V", service, updateChannel )
    shutdownChannel := make(chan bool)
    go func() {
        catalog := r.Client.Catalog()
        killOff := false
        waitIndex := uint64(0)
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
            if waitIndex == 0 {
                /* step: lets get the wait index */
                _, meta, err := catalog.Service(service.Name, "", &consulapi.QueryOptions{})
                if err != nil {
                    glog.Errorf("WatchServices() failed to grab the service: %s fron consul, error: %s", service, err)
                    time.Sleep(5 * time.Second)
                } else {
                    /* update the wait index for this service */
                    waitIndex = meta.LastIndex
                    Verbose("WatchServices() service: %s, index %d", service, meta.LastIndex )
                }
            }
            /* step: build the query - make sure we have a timeout */
            queryOptions := &consulapi.QueryOptions{WaitIndex: r.WaitIndex, WaitTime: defaultWaitTime}

            /* step: making a blocking watch call for changes on the service */
            _, meta, err := catalog.Service(service.Name, "", queryOptions)
            if err != nil {
                glog.Errorf("Failed to wait for service to change, error: %s", err)
                waitIndex = 0
                time.Sleep(5 * time.Second)
            } else {
                if killOff { continue }

                /* step: if the wait and last index are the same, we can continue */
                if r.WaitIndex == meta.LastIndex {
                    Verbose("The WaitIndea and LastIndex are the same, skipping")
                    continue
                }
                /* step: update the index */
                waitIndex = meta.LastIndex
                /* step: send the update upstream */
                Verbose("WatchServices() service: %s changes; sending upstream", service )
                updateChannel <- service
            }
        }
    }()
    return shutdownChannel, nil
}

func (r *ConsulServiceAgent) GetService(svc *consulapi.CatalogService) (service Service) {
    service.ID = svc.ServiceID
    service.Name = svc.ServiceName
    service.Address = svc.Address
    service.Port = svc.ServicePort
    service.Tags = svc.ServiceTags
    return
}
