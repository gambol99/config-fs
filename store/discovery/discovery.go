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
	"flag"
	"net/url"

	"github.com/golang/glog"
)

var (
	InvalidProviderErr = errors.New("Invalid provider name, does not exist")
	discovery_url      *string
)

const VERBOSE_LEVEL = 6

func init() {
	discovery_url = flag.String("discovery", "", "the service discovery backend being used")
}

type ServiceUpdateChannel chan string


/* The Discovery service is a binding between X service discovery providers (i.e. consul, skydns, discoverd etc) */
type Discovery interface {
	/* Get a service from discovery */
	Service(service string) (Service,error)
	/* Get a list of the services */
	Services() ([]Service, error)
	/* Retrieve a list of endpoints for a service */
	Endpoints(service string) ([]Endpoint, error)
	/* Watch for changes on a service and report back */
	Watch(service string) error
	/* Close the service down */
	Close() error
}

func NewDiscovery(channel ServiceUpdateChannel) (Discovery, error) {
	/* step: if the discovery url is not set, we can return a dummy provider */
	if *discovery_url == "" {
		return nil, nil
	} else {
		if uri, err := url.Parse(*discovery_url); err != nil {
			glog.Errorf("Failed to parse the discovery url: %s, error: %s", *discovery_url, err)
			return nil, err
		} else {
			switch uri.Scheme {
			case "consul":
				service, err := NewConsulServiceAgent(uri, channel)
				if err != nil {
					return nil, err
				}
				return service, nil
			}
		}
	}
	return nil, errors.New("Failed to create discovery agent for: " + *discovery_url)
}
