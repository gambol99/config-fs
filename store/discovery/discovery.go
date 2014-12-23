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
    "flag"
    "sync"
    "errors"

    "github.com/gambol99/config-store/discovery/agent"
    "github.com/golang/glog"
)

var (
    discovery_url *string
)

var InvalidProviderErr = errors.New("Invalid provider name, does not exist" )

func init() {
    discovery_url = flag.String("-discovery", "", "the service discovery backend being used" )
}

/*
    The Discovery service is a binding between X service discovery providers (i.e. consul, skydns, discoverd etc)
 */
type Discovery interface {
    /* A list of providers supported */
    ListProviders() []string
    /* Retrieve a list of services */
    FindServices(provider string, name string) ([]agent.Service, error)
    /* Watch for changes on a service and report back */
    WatchService(provider string, service *agent.Service, updateChannel chan *agent.Service) (chan bool, error)
    /* Close the service down */
    Close() error
}

/*
    An implementation of the above interface
 */
type DiscoveryService struct {
    sync.RWMutex
    /* A map of the providers */
    Providers map[string]DiscoveryAgent
    /* A map of stop channels for the service watches */
    StopChannels map[string]chan bool
}

func (r *DiscoveryService) ListProviders() []string {
    r.RLock()
    defer r.RUnlock()
    list := make([]string,0)
    if len( r.Providers ) <= 0 {
        return list
    }
    for name, _ := range r.Providers {
        list = append(list, name )
    }
    return list
}

func (r *DiscoveryService) FindServices(provider string, name string) ([]agent.Service, error) {
    r.RLock()
    defer r.RUnlock()
    if provider, found := r.Providers[provider]; found {
        if services, err := provider.FindServices(name); err != nil {
            glog.Errorf("FindServices() provider: %s, service: %s, failed with error: %s", provider, name, err )
            return nil, err
        } else {
            glog.V(3).Infof("FindServices: provider: %s, found %d services", provider, len(services) )
            return services, nil
        }
    } else {
        return nil, InvalidProviderErr
    }
}

func (r *DiscoveryService) WatchService(provider string, service *agent.Service, updateChannel chan *agent.Service) (chan bool, error) {
    r.Lock()
    defer r.Unlock()
    if provider, found := r.Providers[provider]; found {
        glog.V(3).Infof("WatchService() provider: %s, service: %s, channel: %V", provider, service, updateChannel )
        /* step: lets create the watch on the service */
        stop_watch_channel, err := provider.WatchServices( service, updateChannel )
        if err != nil {
            glog.Errorf("WatchService() failed to watch service: %s, provider: %s", service, provider )
            return nil, err
        }
        /* step: add the stop channel to the stop channel map */
        r.StopChannels[service.Name] = stop_watch_channel
        return stop_watch_channel, nil
    } else {
        return nil, InvalidProviderErr
    }
}

func (r *DiscoveryService) Close() error {
    glog.Infof("Close() closing down the discovery service")
    r.Lock()
    defer r.Unlock()
    for name, stop_channel := range r.StopChannels {
        glog.V(2).Infof("Close() closing the watch on the service: %s", name )
        go func() {
            stop_channel <- true
        }()
    }
    return nil
}

