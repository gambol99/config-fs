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
	"net/url"

	consulapi "github.com/armon/consul-api"
	"github.com/golang/glog"
)

func NewConsulServiceAgent(uri *url.URL) (DiscoveryAgent, error) {
	glog.V(3).Infof("Creating a Consul Discovery Agent, url: %s", uri)
	config := consulapi.DefaultConfig()
	config.Address = uri.Host
	client, err := consulapi.NewClient(config)
	if err != nil {
		glog.Errorf("Failed to create the Consul Client, error: %s", err)
		return nil, err
	}
	agent := new(ConsulServiceAgent)
	agent.Client = client
	return agent, nil
}
