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
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	CONSUL_URL = "http://127.0.0.1:8500"
)

var (
	agent Discovery
)

func TestSetup(t *testing.T) {
	location, err := url.Parse(CONSUL_URL)
	assert.Nil(t, err, "unable to parse the consul url, error: %s", err)
	agent, err = NewConsulServiceAgent(location, make(ServiceUpdateChannel, 10))
	if err != nil {
		t.Fatalf("unable to create a consul client, error: %s", err)
	}
	if agent == nil {
		t.Fatalf("the consul agent should not be nil")
	}
}

func TestService(t *testing.T) {
	_, err := agent.Service("test_server")
	assert.NotNil(t, err, "we should have recieve an error here but didn't")
}
