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
	"reflect"
	"sort"
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
				"base":       path.Base,
				"contained":  config.Contains,
				"dir":        path.Dir,
				"endpoints":  config.FindEndpoints,
				"endpointsl": config.FindEndpointsList,
				"get":        config.GetKeyPair,
				"getenv":     os.Getenv,
				"getl":       config.GetList,
				"gets":       config.GetKerPairs,
				"getv":       config.GetValue,
				"join":       strings.Join,
				"json":       config.UnmarshallJSON,
				"jsona":      config.UnmarshallJSONArray,
				"prefix":     strings.HasPrefix,
				"service":    config.FindService,
				"services":   config.FindServices,
				"split":      strings.Split,
				"suffix":     strings.HasSuffix}

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
				if r.discovery != nil {
					r.discovery.Close()
				}
				r.store.Close()
			}
		}
	}()
}

func (r *DynamicConfig) RetrieveKeyValue(key string, watch bool) (*kv.Node, error) {
	if node, err := r.store.Get(key); err != nil {
		glog.Errorf("Failed to reteive the content from key: %s, error: %s", key, err)
		return nil, err
	} else {
		if node.IsDir() {
			return nil, errors.New("The the key: " + key + " is a directory not a value" )
		}
		if watch {
			r.store.Watch(key)
		}
		return node, nil
	}
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
func (r *DynamicConfig) FindService(service string) (discovery.Service, error) {
	glog.V(VERBOSE_LEVEL).Infof("FindService() service: %s", service)
	/* step: make sure a discovery service exists */
	if r.discovery == nil {
		return discovery.Service{}, errors.New("No service discovery service was specified in config")
	}

	/* step: we add a watch to the service via the agent, assuming there isn't one already */
	if err := r.discovery.Watch(service); err != nil {
		glog.Errorf("Failed to add a watch for service: %s, error: %s", service, err)
		return discovery.Service{}, err
	}

	/* step: we attempt to find the exist, though it might not exist yet */
	if service, err := r.discovery.Service(service); err != nil {
		glog.Errorf("Failed to find the service: %s, error: %s", service, err)
		return discovery.Service{}, nil
	} else {
		return service, nil
	}
}

func (r *DynamicConfig) FindServices() ([]discovery.Service, error) {
	/* step: make sure a discovery service exists */
	if r.discovery == nil {
		return nil, errors.New("No service discovery service was specified in config")
	}
	/* step: get a list of services */
	if services, err := r.discovery.Services(); err != nil {
		glog.Errorf("Failed to retrieve a list of services from discovery provider, error: %s", err)
		return nil, err
	} else {
		/* step: return the services */
		return services, nil
	}
}

func (r *DynamicConfig) FindEndpoints(service string) ([]discovery.Endpoint, error) {
	glog.V(VERBOSE_LEVEL).Infof("FindEndpoints() service: %s", service)

	/* step: we add a watch to the service via the agent, assuming there isn't one already */
	if err := r.discovery.Watch(service); err != nil {
		glog.Errorf("Failed to add a watch for service: %s, error: %s", service, err)
		return nil, err
	}

	/* step: find some endpoints */
	if endpoints, err := r.discovery.Endpoints(service); err != nil {
		glog.Errorf("Failed to find service: %s, error: %s", service, err)
		return nil, err
	} else {
		return endpoints, nil
	}
}

func (r *DynamicConfig) FindEndpointsList(service string) ([]string, error) {
	glog.V(VERBOSE_LEVEL).Infof("FindEndpointsList() service: %s", service)
	if endpoints, err := r.FindEndpoints(service); err != nil {
		glog.Errorf("Failed to find service: %s, error: %s", service, err)
		return nil, err
	} else {
		list := make([]string, 0)
		for _, endpoint := range endpoints {
			list = append(list, fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port))
		}
		return list, nil
	}
}

func (r *DynamicConfig) GetKeyPair(key string) (kv.Node, error) {
	if node, err := r.RetrieveKeyValue(key, true); err != nil {
		return kv.Node{}, err
	} else {
		return *node, nil
	}
}

func (r *DynamicConfig) GetValue(key string) string {
	if node, err := r.RetrieveKeyValue(key, true); err != nil {
		return ""
	} else {
		return node.Value
	}
}

func (r *DynamicConfig) GetKerPairs(path string) ([]*kv.Node, error) {
	if paths, err := r.store.List(path); err != nil {
		glog.Errorf("Failed to get a list of keys under directory: %s, error: %s", path, err)
		return nil, err
	} else {
		/* step: we add a watch on the directory */
		r.store.Watch(path)
		/* step: return the result */
		return paths, nil
	}
}

func (r *DynamicConfig) GetList(path string) ([]string, error) {
	if paths, err := r.store.List(path); err != nil {
		glog.Errorf("Failed to get a list of keys under directory: %s, error: %s", path, err)
		return nil, err
	} else {
		/* step: we add a watch on the directory */
		r.store.Watch(path)
		/* step: compose the list of strings */
		list := make([]string, 0)
		for _, node := range paths {
			if node.IsFile() {
				list = append(list, node.Path)
			}
		}
		/* step: we sort the strings */
		sort.Strings(list)
		return list, nil
	}
}

func (r *DynamicConfig) Contains(list interface{}, elem interface{}) bool {
	v := reflect.ValueOf(list)
	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == elem {
			return true
		}
	}
	return false
}

func (r *DynamicConfig) UnmarshallJSONArray(key string) ([]interface{}, error) {
	if node, err := r.RetrieveKeyValue(key, true); err != nil {
		return nil, err
	} else {
		var content []interface{}
		if err := json.Unmarshal([]byte(node.Value), &content); err != nil {
			glog.Errorf("Failed to unmarshall the json data into an array, error: %s", err)
			return nil, err
		}
		return content, nil
	}
}

func (r *DynamicConfig) UnmarshallJSON(key string) (map[string]interface{}, error) {
	if node, err := r.RetrieveKeyValue(key, true); err != nil {
		return nil, err
	} else {
		var content map[string]interface{}
		if err := json.Unmarshal([]byte(node.Value), &content); err != nil {
			glog.Errorf("Failed to unmarshall the json content: %s, error: %s", node.Value, err)
			return nil, err
		}
		return content, nil
	}
}

