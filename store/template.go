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

package store

import (
	"encoding/json"
	"text/template"
	"bytes"

	"github.com/gambol99/config-fs/store/kv"
	"github.com/gambol99/config-fs/store/discovery"
	"github.com/gambol99/config-fs/store/discovery/agent"
	"github.com/golang/glog"
)

type TemplatedResource interface {
	/* watch the template for change and send notification via channel */
	WatchTemplate(channel TemplateUpdateChannel)
	/* render the content of the template */
	Render() (string,error)
	/* shutdown and release the assets */
	Close()
}

type TemplatedConfig struct {
	/* the path of the file */
	path string
	/* the actual template */
	template *template.Template
	/* the discovery service */
	discovery discovery.Discovery
	/* the k/v store */
	store kv.KVStore
	/* the keys we are watching */
	watchingKeys map[string]interface {}
	/* the services we are watching */
	watchingServices map[string][]agent.Service

	/* the channel for listening to events */
	storeUpdateChannel kv.NodeUpdateChannel
	/* service update channel */
	serviceUpdateChannel agent.ServiceUpdateChannel
	/* stop channel */
	stopChannel chan bool
}

func NewResourceTemplate(path, content string, store kv.KVStore, discovery discovery.Discovery) (TemplatedResource,error) {
	glog.Infof("Creating new template, path: %s", path )
	t := new(TemplatedConfig)
	t.path = path
	t.store = store
	t.discovery = discovery
	/* step: create the function map for this template */
	functionMap := template.FuncMap {
		"service":  t.FindService,
		"getv": t.GetKeyValue,
		"json": t.MarshallJSON }
	if tpl, err := template.New(path).Funcs(functionMap).Parse(content); err != nil {
		glog.Errorf("Failed to create the template: %s, error: %s", path, err )
		return nil, err
	} else {
		t.template = tpl
		t.watchingKeys = make(map[string]interface {},5)
		t.watchingServices = make(map[string][]agent.Service,5)
		t.storeUpdateChannel = make(kv.NodeUpdateChannel,5)
		t.serviceUpdateChannel = make(agent.ServiceUpdateChannel,5)
	}
	return t, nil
}

func (r TemplatedConfig) Close() {
	glog.Infof("Closing the resources for template: %s", r.path )
	r.stopChannel <- true
}

func (r *TemplatedConfig) Render() (string,error) {
	var content bytes.Buffer
	glog.V(VERBOSE_LEVEL).Infof("Render() rendering the terplate")
	if err := r.template.Execute(&content,nil); err != nil {
		glog.Errorf("Failed to render the content of file: %s, error: %s", r.path, err)
		return "", err
	}
	return content.String(), nil
}

func (r *TemplatedConfig) WatchTemplate(channel TemplateUpdateChannel) {
	r.stopChannel = make(chan bool, 1)
	go func() {
		for {
			select {
			case event := <- r.storeUpdateChannel:
				glog.V(VERBOSE_LEVEL).Infof("Template: %s, event: %s", r.path, event )


				channel <- r.path
			case event := <- r.serviceUpdateChannel:
				glog.V(VERBOSE_LEVEL).Infof("Template: %s, event: %s", r.path, event )

				channel <- r.path
			case <- r.stopChannel:
				glog.Infof("Shutting down the template for: %s", r.path )
				/* @@TODO we need to remove the keys from being watched and remove the servics */
			}
		}
	}()
}

/* ============ T E M P L A T E   M E T H O D S  ============== */

func (r *TemplatedConfig) GetKeyValue(key string) string {
	glog.V(VERBOSE_LEVEL).Infof("getv() key: %s", key )
	/* step: check if we have the value in the map */
	if content, found := r.watchingKeys[key]; found {
		glog.V(VERBOSE_LEVEL).Infof("get() key: %s found in the cache", key )
		return content.(string)
	} else {
		/* step: we need to grab the key from the store and store */
		content, err := r.store.Get(key)
		if err != nil {
			glog.Errorf("Failed to get the key: %s, error: %s", key, err )
			return ""
		}
		/* step: check if the key is presently being watched */
		r.watchingKeys[key] = content
	}

	return ""
}

func (r *TemplatedConfig) MarshallJSON(content string) interface {} {
	var json_data map[string]interface {}
	if err := json.Unmarshal([]byte(content),&json_data); err != nil {
		glog.Errorf("Failed to unmarshall the json data, value: %s, error: %s", content, err )
		return ""
	} else {
		return json_data
	}
}

func (r *TemplatedConfig) FindService(provider string, service string) []agent.Service {
	glog.V(VERBOSE_LEVEL).Infof("service() provider: %s, service: %s", provider, service )



	return nil
}
