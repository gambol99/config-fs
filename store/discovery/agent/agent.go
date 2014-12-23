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
	"github.com/golang/glog"
)

const (
	AGENT_VERBOSE_LEVEL = 6
)

func Verbose(message string, args ...interface {}) {
	glog.V(AGENT_VERBOSE_LEVEL).Infof(message, args)
}

type DiscoveryAgent interface {
	/* search for service which match the filter */
	FindServices(filter string) ([]Service, error)
	/* notify my channel of updates to services */
	WatchServices(services *Service, updateChannel chan *Service) (chan bool, error)
}

type Service struct {
	ID 		string
	/* the name of the service */
	Name    string
	/* the ip address of the service */
	Address string
	/* the port the service is running on */
	Port 	uint
	/* any tags related to the service */
	Tags 	[]string
}

