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
	"text/template"

	"github.com/gambol99/config-fs/store/discovery"
	"github.com/gambol99/config-fs/store/kv"
	"github.com/golang/glog"
)

type DynamicResource interface {
	/* watch the template for change and send notification via channel */
	Watch(channel DynamicUpdateChannel)
	/* render the content of the template */
	Render() (string, error)
	/* shutdown and release the assets */
	Close()
}

type DynamicConfig struct {
	/* the path of the file */
	path string
	/* the actual template */
	template *template.Template
	/* the discovery service */
	discovery discovery.Discovery
	/* the k/v store */
	store kv.KVStore
	/* the keys we are watching */
	watchingKeys map[string]interface{}
	/* the services we are watching */
	watchingServices map[string][]discovery.Endpoint
	/* the channel for listening to events */
	storeUpdateChannel kv.NodeUpdateChannel
	/* service update channel */
	serviceUpdateChannel discovery.ServiceUpdateChannel
	/* stop channel */
	stopChannel chan bool
}

func NewDynamicResource(path, content string, store kv.KVStore) (DynamicResource, error) {
	glog.Infof("Creating new template, path: %s", path)
	t := new(DynamicConfig)
	t.path = path
	t.store = store

	/* step: create a discovery agent */
	if agent, err := discovery.NewConsulServiceAgent(); err != nil {
		glog.Errorf("Failed to create the discovery agent, error: %s", err)
		return nil, err
	} else {
		t.discovery = agent
	}

	/* step: create the function map for this template */
	functionMap := template.FuncMap{
		"service": t.FindService,
		"getv":    t.GetKeyValue,
		"json":    t.UnmarshallJSON}
	if tpl, err := template.New(path).Funcs(functionMap).Parse(content); err != nil {
		glog.Errorf("Failed to create the template: %s, error: %s", path, err)
		return nil, err
	} else {
		t.template = tpl
		t.watchingKeys = make(map[string]interface{}, 0)
		t.watchingServices = make(map[string][]discovery.Endpoint, 0)
		t.storeUpdateChannel = make(kv.NodeUpdateChannel, 5)
		t.serviceUpdateChannel = make(discovery.ServiceUpdateChannel, 5)
	}
	return t, nil
}

func (r DynamicConfig) Close() {
	glog.Infof("Closing the resources for template: %s", r.path)
	r.stopChannel <- true
}

func (r *DynamicConfig) Render() (string, error) {
	var content bytes.Buffer
	glog.V(VERBOSE_LEVEL).Infof("Render() rendering the template: %s", r.path)
	if err := r.template.Execute(&content, nil); err != nil {
		glog.Errorf("Failed to render the content of file: %s, error: %s", r.path, err)
		return "", err
	}
	/* step: return the rendered content minus the prefix */
	return content.String()[len(DYNAMIC_PREFIX):], nil
}

func (r *DynamicConfig) Watch(channel DynamicUpdateChannel) {
	r.stopChannel = make(chan bool, 1)
	glog.V(VERBOSE_LEVEL).Infof("Adding a listener for the template: %s, channel: %v", r.path, channel)
	go func() {
		for {
			select {
			case event := <-r.storeUpdateChannel:
				glog.V(VERBOSE_LEVEL).Infof("Template: %s, event: %s", r.path, event)
				r.HandleNodeEvent(event, channel)
			case service := <-r.serviceUpdateChannel:
				glog.V(VERBOSE_LEVEL).Infof("Template: %s, event: %s", r.path, service)
				r.HandleServiceEvent(service, channel)
			case <-r.stopChannel:
				glog.Infof("Shutting down the template for: %s", r.path)
				/* @@TODO we need to remove the keys from being watched and remove the servics */
			}
		}
	}()
}

/* ============ EVENT HANDLERS ================================ */

func (r *DynamicConfig) HandleNodeEvent(event kv.NodeChange, channel DynamicUpdateChannel) {
	glog.Infof("The key: %s, in template: %s has change", event, r.path)

	channel <- r.path
}

func (r *DynamicConfig) HandleServiceEvent(service string, channel DynamicUpdateChannel) {
	glog.Infof("The service: %s in template: %s has changed, pulling the list", service, r.path)
	if endpoints, err := r.discovery.ListEndpoints(service); err != nil {
		glog.Errorf("Failed to update the service: %s, in template: %s, error: %s", service, r.path, err)
	} else {
		glog.V(VERBOSE_LEVEL).Infof("Template: %s, endpoints: %s", r.path, endpoints)
		/* step: update the endpoints for the services */
		r.watchingServices[service] = endpoints
		go func() {
			channel <- r.path
		}()
	}
}

/* ============ TEMPLATE METHODS ============================== */

func (r *DynamicConfig) GetKeyValue(key string) string {
	glog.V(VERBOSE_LEVEL).Infof("getv() key: %s", key)
	/* step: check if we have the value in the map */
	if content, found := r.watchingKeys[key]; found {
		glog.V(VERBOSE_LEVEL).Infof("get() key: %s found in the cache", key)
		return content.(string)
	} else {
		/* step: we need to grab the key from the store and store */
		content, err := r.store.Get(key)
		if err != nil {
			glog.Errorf("Failed to get the key: %s, error: %s", key, err)
			return ""
		}
		/* step: check if the key is presently being watched */
		r.watchingKeys[key] = content.Value
		/* return the content */
		return content.Value
	}
	return ""
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

func (r *DynamicConfig) FindService(service string) []discovery.Endpoint {
	glog.V(VERBOSE_LEVEL).Infof("FindService() provider: %s, service: %s", service)
	services := make([]discovery.Endpoint, 0)

	if list, found := r.watchingServices[service]; found {
		glog.V(VERBOSE_LEVEL).Infof("FindService() found service: %s in cache", service)
		return list
	} else {
		/* must be the first time we have run - we need to grab the services and place a watch on the service */
		if stopChannel, err := r.discovery.WatchService(service, r.serviceUpdateChannel); err != nil {
			glog.Errorf("Failed to add a watch for service: %s, error: %s", service, err)
			return services
		} else {
			var _ = stopChannel
		}
		/* step: we get a list of the services */
		list, err := r.discovery.ListEndpoints(service)
		if err != nil {
			glog.Errorf("Failed to retrieve a list of services for service: %s, error: %s", service, err)
			return services
		}
		glog.V(VERBOSE_LEVEL).Infof("Found %d endpoints for service: %s, template: %s, endpoints: %s", len(list), service, r.path, list)
		/* step: update the map */
		r.watchingServices[service] = list
		return list
	}
	return nil
}
