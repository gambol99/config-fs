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
	"net/url"

	marathon "github.com/gambol99/go-marathon"
	"github.com/golang/glog"
)

const (
	DEFAULT_MARATHON_PORT	  = 9900
	DEFAULT_MARATHON_INTEFACE = "eth0"
)

var (
	// The port we should listen on for events from marathon
	marathon_port int

	// The interface we should use to listen out for events
	marathon_interface string
)

func init() {
	flag.IntVar(&marathon_port, "marathon_port", DEFAULT_MARATHON_PORT, "the port we should use to listen for marathon events")
	flag.StringVar(&marathon_interface, "marathon_port", DEFAULT_MARATHON_INTEFACE, "the port we should use to listen for marathon events")
}

type MarathonServiceAgent struct {
	// The channel we send updates on
	update_channel ServiceUpdateChannel

	// The client for marathon
	client *marathon.Client


}

func NewMarathonServiceAgent(uri *url.URL, channel ServiceUpdateChannel) (Discovery, error) {
	glog.V(3).Infof("Creating a Marathon Discovery Agent, url: %s", uri.String())

	// step: we create the configuration
	config := marathon.NewDefaultConfig()
	config.URL = uri.Host
	config.EventsPort = marathon_port
	config.EventsInterface = marathon_interface

	// step: create the marathon client
	service := new(MarathonServiceAgent)
	if client, err := marathon.NewClient(config); err != nil {
		glog.Errorf("Failed to create the marathon client, error: %s", err)
		return nil, err
	} else {
		service.client = client
		service.update_channel = channel
	}
	return service, nil
}

func (r *MarathonServiceAgent) Service(service string) (Service, error) {
	if application, err := r.client.Application(service); err != nil {
		glog.Errorf("Failed to retrieve the application from marathon, error: %s", err)
		return nil, err
	} else {
		return Service{application.ID, nil}, nil
	}
}

func (r *MarathonServiceAgent) Services() ([]Service, error) {
	// step: retrieve a list of applications from marathon
	if applications, err := r.client.Applications(); err != nil {
		glog.Errorf("Failed to retrieve a list application from marathon, error: %s", err)
		return nil, err
	} else {
		services := make([]Service, 0)
		for _, application := range applications.Apps {
			services = append(services, Service{application.ID}, nil)
		}
		return services, nil
	}
}

func (r *MarathonServiceAgent) Endpoints(service string) ([]Endpoint, error) {

}

func (r *MarathonServiceAgent) Watch(service string) error {
	update_channel := make(marathon.EventsChannel)


}

func (r *MarathonServiceAgent) Close() error {

}


