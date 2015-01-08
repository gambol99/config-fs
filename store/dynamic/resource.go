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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
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
	sync.RWMutex
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
	/* the channel for listening to events */
	storeUpdateChannel kv.NodeUpdateChannel
	/* service update channel */
	serviceUpdateChannel discovery.ServiceUpdateChannel
	/* stop channel */
	stopChannel chan bool
}

func NewDynamicResource(filename, content string) (DynamicResource, error) {
	glog.Infof("Creating new dynamic config, path: %s", filename)
	config := new(DynamicConfig)
	config.path = filename
	config.storeUpdateChannel = make(kv.NodeUpdateChannel, 5)
	/* step: we create a new kv client for the resource */
	if agent, err := kv.NewKVStore(config.storeUpdateChannel); err != nil {
		glog.Errorf("Failed to create a kv agent, error: %s", err)
		return nil, err
	} else {
		config.store = agent
		config.stopChannel = make(chan bool)
		config.serviceUpdateChannel = make(discovery.ServiceUpdateChannel, 5)
		/* step: create a discovery agent */
		if disx, err := discovery.NewDiscovery(config.serviceUpdateChannel); err != nil {
			agent.Close()
			return nil, err
		} else {
			config.discovery = disx
			/* step: create the function map for this template */
			functionMap := template.FuncMap{
				"service":   config.FindService,
				"endpoints": config.FindEndpoints,
				"get":       config.GetKeyPair,
				"getv":      config.GetValue,
				"getr":      config.GetList,
				"json":      config.UnmarshallJSON,
				"jsona":     config.UnmarshallJSONArray,
				"base":      path.Base,
				"dir":       path.Dir,
				"split":     strings.Split,
				"getenv":    os.Getenv,
				"join":      strings.Join}

			if resource, err := template.New(filename).Funcs(functionMap).Parse(content); err != nil {
				glog.Errorf("Failed to parse the dynamic config: %s, error: %s", config.path, err)
				return nil, err
			} else {
				config.template = resource
			}
			return config, nil
		}
	}
}

func (r DynamicConfig) Close() {
	glog.Infof("Closing the resources for dynamic config: %s", r.path)
	r.stopChannel <- true
}

func (r *DynamicConfig) Watch(channel DynamicUpdateChannel) {
	r.stopChannel = make(chan bool)
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
				r.discovery.Close()
				r.store.Close()
			}
		}
	}()
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

/* Dynamic Config templating functions */
func (r *DynamicConfig) FindService(service string) ([]discovery.Endpoint, error) {
	glog.V(VERBOSE_LEVEL).Infof("FindService() service: %s", service)

	/* step: make sure a discovery service exists */
	if r.discovery == nil {
		return nil, errors.New("No service discovery service was specified in config")
	}

	/* step: we add a watch to the service via the agent, assuming there isn't one already */
	if err := r.discovery.WatchService(service); err != nil {
		glog.Errorf("Failed to add a watch for service: %s, error: %s", service, err)
		return make([]discovery.Endpoint, 0), nil
	}
	/* step: we retrieve a list of the endpoints from the agent */
	if list, err := r.discovery.ListEndpoints(service); err != nil {
		glog.Errorf("Failed to retrieve a list of services for service: %s, error: %s", service, err)
		return make([]discovery.Endpoint, 0), nil
	} else {
		glog.V(VERBOSE_LEVEL).Infof("Found %d endpoints for service: %s, dynamic config: %s, endpoints: %s", len(list), service, r.path, list)
		return list, nil
	}
}

func (r *DynamicConfig) FindEndpoints(service string) ([]string, error) {
	glog.V(VERBOSE_LEVEL).Infof("FindEndpoints() service: %s", service)
	/* step: a list of endpoints <address>:<port> */
	list := make([]string, 0)
	/* step: find some endpoints */
	if endpoints, err := r.FindService(service); err != nil {
		glog.Errorf("Failed to find service: %s, error: %s", service, err)
		return nil, err
	} else {
		for _, endpoint := range endpoints {
			list = append(list, fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port))
		}
		return list, nil
	}
}

func (r *DynamicConfig) ListDirectory(directory string) ([]string, error) {
	paths := make([]string, 0)
	if paths, err := r.store.Paths(directory, &paths); err != nil {
		glog.Errorf("Failed to generate list of keys under directory: %s", directory)
		return paths, err
	} else {
		/* step: we place a watch on the directory */
		r.store.Watch(directory)
		/* step: we get the paths */
		return paths, nil
	}
}

func (r *DynamicConfig) GetKeyPair(key string) (kv.Node, error) {
	if node, err := r.store.Get(key); err != nil {
		glog.Errorf("Failed to get the key: %s, error: %s", key, err)
		return kv.Node{}, err
	} else {
		return *node, nil
	}
}

func (r *DynamicConfig) GetValue(key string) string {
	if content, err := r.store.Get(key); err != nil {
		glog.Errorf("Failed to get the key: %s, error: %s", key, err)
		return ""
	} else {
		/* step: we add a watch on the key */
		r.store.Watch(key)
		/* return the content */
		return content.Value
	}
}

func (r *DynamicConfig) GetList(path string) ([]string,error) {
	if paths, err := r.store.List(path); err != nil {
		glog.Errorf("Failed to get a list of keys under directory: %s, error: %s", path, err)
		return nil, err
	} else {
		list := make([]string, 0)
		for _, node := range paths {
			if node.IsFile() {
				list = append(list, node.Path)
			}
		}
		return list, nil
	}
}

func (r *DynamicConfig) UnmarshallJSONArray(content string) ([]interface{}, error) {
	var ret []interface{}
	if err := json.Unmarshal([]byte(content), &ret); err != nil {
		glog.Errorf("Failed to unmarshall the json data into an array, error: %s", err)
		return nil, err
	}
	return ret, nil
}

func (r *DynamicConfig) UnmarshallJSON(content string) (map[string]interface{}, error) {
	var json_data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &json_data); err != nil {
		glog.Errorf("Failed to unmarshall the json data, value: %s, error: %s", content, err)
		return nil, err
	} else {
		return json_data, nil
	}
}
