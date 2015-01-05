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
	"sync"
	"strings"

	"github.com/gambol99/config-fs/store/kv"
	"github.com/golang/glog"
)

const (
	DYNAMIC_PREFIX  = "$TEMPLATE$"
	VERBOSE_LEVEL     = 5
)

type DynamicUpdateChannel chan string

type DynamicStore interface {
	IsDynamic(path string) (DynamicResource, bool)
	IsDynamicContent(path, content string) bool
	Create(path, content string, channel DynamicUpdateChannel) (string, error)
	Delete(path string)
}

type DynamicStoreImpl struct {
	/* a lock on the resources */
	sync.RWMutex
	/* a map of the templated resources */
	resources map[string]DynamicResource
	/* the key value store */
	backend kv.KVStore
	/* the prefix used for check if content is dynamic */
	prefix string
}

func NewDynamicStore(prefix string, backend kv.KVStore) (DynamicStore) {
	service := new(DynamicStoreImpl)
	service.resources = make(map[string]DynamicResource,0)
	service.prefix = DYNAMIC_PREFIX
	service.backend = backend
	if prefix != "" {
		service.prefix = prefix
	}
	return service
}

/* check if a resource path is a dynamic resource */
func (r *DynamicStoreImpl) IsDynamic(path string) (DynamicResource, bool) {
	r.RLock()
	defer r.RUnlock()
	if resource, found := r.resources[path]; found {
		return resource, true
	}
	return nil, false
}

/* check if the content of a resource is a dynamic content */
func (r *DynamicStoreImpl) IsDynamicContent(path, content string) bool {
	if strings.HasPrefix(content, r.prefix) {
		glog.V(VERBOSE_LEVEL).Infof("Found templated content in file: %s", path)
		return true
	}
	return false
}

func (r *DynamicStoreImpl) Create(path, content string, channel DynamicUpdateChannel) (string, error) {
	glog.V(VERBOSE_LEVEL).Infof("Creating a new template, path: %s", path )
	if _, found := r.IsDynamic(path); found {
		glog.Errorf("The template: %s already exist, we can skip creation", path)
		return "", nil
	}
	/* step: we need to create a template for this
	- we read in the template content
	- we generate the content
	- we create watches on the keys / services
	- and we update the store with a notification when the template changes
	*/
	if resource, err := NewDynamicResource(path, content, r.backend ); err != nil {
		glog.Errorf("Failed to create the templated resournce: %s, error: %s", path, err)
		return "", err
	} else {
		/* step: we generate the templated content ready to return */
		if content, err := resource.Render(); err != nil {
			glog.Errorf("Failed to render the template: %s, error: %s", path, err)
			return "", err
		} else {
			/* step: we need to listen out to events from the template */
			resource.Watch(channel)
			/* step: we need to add the map */
			r.Add(path, resource)
			/* return the content of the template */
			return content, nil
		}
	}
}

func (r *DynamicStoreImpl) Add(path string, resource DynamicResource) {
	r.Lock()
	defer r.Unlock()
	r.resources[path] = resource
}

func (r *DynamicStoreImpl) Delete(path string) {
	r.Lock()
	defer r.Unlock()
	if resource, found := r.resources[path]; found {
		/* step: close the resource */
		resource.Close()
		/* step: we remove from the map */
		delete(r.resources, path)
	}
}
