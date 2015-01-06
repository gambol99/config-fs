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

package dynamic

import (
	"bytes"
	"encoding/json"
	"sync"
	"text/template"

	"github.com/gambol99/config-fs/store/discovery"
	"github.com/gambol99/config-fs/store/kv"
	"github.com/golang/glog"
)

type DynamicResource interface {
	/* watch the dynamic config for changes and send notification via channel */
	Watch(channel DynamicUpdateChannel)
	/* get the content of the config */
	Content(forceRefresh bool) (string, error)
	/* shutdown and release the assets */
	Close()
}

type DynamicConfig struct {
	sync.Mutex
	/* the path of the file */
	path string
	/* the actual template */
	template *template.Template
	/* the discovery service */
	discovery discovery.Discovery
	/* the k/v store */
	store kv.KVStore
	/* the generate content */
	content string
	/* a map of keys being watched */
	stopKeyChannels map[string]chan bool
	/* the channel for listening to events */
	storeUpdateChannel kv.NodeUpdateChannel
	/* service update channel */
	serviceUpdateChannel discovery.ServiceUpdateChannel
	/* stop channel */
	stopChannel chan bool
}

func NewDynamicResource(path, content string, store kv.KVStore) (DynamicResource, error) {
	glog.Infof("Creating new dynamic config, path: %s", path)
	config := new(DynamicConfig)
	config.path = path
	config.store = store
	config.stopChannel = make(chan bool, 0)
	config.stopKeyChannels = make(map[string]chan bool, 0)
	config.storeUpdateChannel = make(kv.NodeUpdateChannel, 5)
	config.serviceUpdateChannel = make(discovery.ServiceUpdateChannel, 5)

	/* step: create a discovery agent */
	if agent, err := discovery.NewConsulServiceAgent(config.serviceUpdateChannel); err != nil {
		return nil, err
	} else {
		config.discovery = agent
	}

	/* step: create the function map for this template */
	functionMap := template.FuncMap{
		"service": config.FindService,
		"getv":    config.GetValue,
		"json":    config.UnmarshallJSON}

	if resource, err := template.New(path).Funcs(functionMap).Parse(content); err != nil {
		return nil, err
	} else {
		config.template = resource
	}
	return config, nil
}

func (r DynamicConfig) Close() {
	glog.Infof("Closing the resources for dynamic config: %s", r.path)
	r.stopChannel <- true

}

func (r *DynamicConfig) Content(forceRefresh bool) (string, error) {
	/* step: get the content from the cache if there and refresh is false */
	if r.content != "" && forceRefresh == false {
		return r.content, nil
	}
	if err := r.Generate(); err != nil {
		glog.Errorf("Failed to generate the dynamic content for config: %s, error: %s", r.path, err)
		return "", err
	} else {
		return r.content, nil
	}
}

func (r *DynamicConfig) Generate() error {
	r.Lock()
	defer r.Unlock()
	if content, err := r.Render(); err != nil {
		glog.Errorf("Failed to re-generate the content for config: %s, error: %s", r.path, err)
		return err
	} else {
		glog.V(VERBOSE_LEVEL).Infof("Updating the content for config: %s", r.path)
		/* step: update the cache copy */
		r.content = content
	}
	return nil
}

func (r *DynamicConfig) Render() (string, error) {
	var content bytes.Buffer
	if err := r.template.Execute(&content, nil); err != nil {
		return "", err
	}
	return content.String()[len(DYNAMIC_PREFIX):], nil
}

func (r *DynamicConfig) Watch(channel DynamicUpdateChannel) {
	r.stopChannel = make(chan bool, 1)
	glog.V(VERBOSE_LEVEL).Infof("Adding a listener for the dynamic config: %s, channel: %v", r.path, channel)
	go func() {
		for {
			select {
			case event := <-r.storeUpdateChannel:
				glog.V(VERBOSE_LEVEL).Infof("Dynamic config: %s, event: %s", r.path, event)
				if err := r.Generate(); err == nil {
					channel <- r.path
				}
			case service := <-r.serviceUpdateChannel:
				glog.V(VERBOSE_LEVEL).Infof("Dynamic config: %s, event: %s", r.path, service)
				if err := r.Generate(); err == nil {
					channel <- r.path
				}
			case <-r.stopChannel:
				glog.Infof("Shutting down the resources for dynamic config: %s", r.path)
				/* step: close all the watches on the key */
				for path, channel := range r.stopKeyChannels {
					glog.V(VERBOSE_LEVEL).Infof("Stopping the k/v watch on key: %s", path)
					channel <- true
				}
			}
		}
	}()
}

func (r *DynamicConfig) HandleNodeEvent(event kv.NodeChange, channel DynamicUpdateChannel) {
	glog.Infof("The key: %s, in dynamic config: %s has changed", event, r.path)
	if err := r.Generate(); err != nil {
		glog.Errorf("Failed to re-generate the content for config: %s, error: %s", r.path, err)
	} else {
		channel <- r.path
	}

}

func (r *DynamicConfig) HandleServiceEvent(service string, channel DynamicUpdateChannel) {
	glog.Infof("The service: %s in dynamic config: %s has changed, pulling the list", service, r.path)
	if content, err := r.Render(); err != nil {
		glog.Errorf("Failed to re-generate the content for config: %s, error: %s", r.path, err)
	} else {
		r.content = content
		go func() {
			channel <- r.path
		}()
	}
}

func (r *DynamicConfig) GetValue(key string) string {
	/* step: we add a watch on the key */
	if stopChannel, err := r.store.Watch(key, r.storeUpdateChannel); err != nil {
		glog.Errorf("Failed to add a watch for key: %s, error: %s", key, err)
		return ""
	} else {
		r.stopKeyChannels[key] = stopChannel
		/* step: we need to grab the key from the store and store */
		if content, err := r.store.Get(key); err != nil {
			glog.Errorf("Failed to get the key: %s, error: %s", key, err)
			return ""
		} else {
			return content.Value
		}
	}
	return ""
}

func (r *DynamicConfig) FindService(service string) []discovery.Endpoint {
	glog.V(VERBOSE_LEVEL).Infof("FindService() service: %s", service)

	/* step: we add a watch to the service via the agent, assuming there isnt one already */
	if err := r.discovery.WatchService(service); err != nil {
		glog.Errorf("Failed to add a watch for service: %s, error: %s", service, err)
		return make([]discovery.Endpoint, 0)
	}
	/* step: we retrieve a list of the endpoints from the agent */
	if list, err := r.discovery.ListEndpoints(service); err != nil {
		glog.Errorf("Failed to retrieve a list of services for service: %s, error: %s", service, err)
		return make([]discovery.Endpoint, 0)
	} else {
		glog.V(VERBOSE_LEVEL).Infof("Found %d endpoints for service: %s, dynamic config: %s, endpoints: %s", len(list), service, r.path, list)
		return list
	}
}

func (r *DynamicConfig) UnmarshallJSON(content string) interface{} {
	var json_data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &json_data); err != nil {
		glog.Errorf("Failed to unmarshall the json data, value: %s, error: %s", content, err)
		return ""
	} else {
		return json_data
	}
}