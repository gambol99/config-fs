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
	"github.com/golang/glog"
)

type DummyServiceImpl struct{}

func NewDummyServiceAgent() Discovery {
	glog.V(3).Infof("Creating a Dummy Service Agent")
	return &DummyServiceImpl{}
}

func (r *DummyServiceImpl) ListEndpoints(string) ([]Endpoint, error) {
	return make([]Endpoint, 0), nil
}

func (r *DummyServiceImpl) WatchService(string) error {
	return nil
}

func (r *DummyServiceImpl) Close() error {
	return nil
}
