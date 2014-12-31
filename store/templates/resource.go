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

package templates

import (
	"sync"
	"errors"

	"github.com/golang/glog"
	"github.com/gambol99/config-fs/store/discovery"
)

const (
	VERBOSE_LEVEL = 5
)

/* the template resource manager interface */
type TemplateManager interface {
	/* check if the path is a template */
	IsTemplate(path string) bool
	/* add a new template to the manager */
	AddTemplate(path string, content string) error
	/* remove a template from the manager */
	RemoveTemplate(path string) error
}

/* the implementation of the above */
type TemplateResources struct {
	/* locking for the manager */
	sync.RWMutex
	/* a map of the current resources */
	resources map[string]*Template
}

type Template struct {
	/* the actual template */
	content	string
}

/* create a new template manager */
func NewTemplateManager(discovery discovery.Discovery) TemplateManager {
	glog.Infof("Creating a new Template Manager, discovey service: %s", discovery )

	return nil
}

/* check if the template exists */
func (r *TemplateResources) IsTemplate(path string) bool {
	r.RLock()
	defer r.RUnlock()
	if _, found := r.resources[path]; found {
		return true
	}
	return false
}

func (r *TemplateResources) AddTemplate(path string, content string) error {
	glog.V(VERBOSE_LEVEL).Infof("Adding a new template: %s, content: %s", path, content )
	if r.IsTemplate(path) {
		glog.Errorf("The template: %s already exist", path )
		return nil
	}
	/* step: we need to create a template for this
		- we read in the template content
		- we generate the content
		- we create watches on the keys / services
		- and we update the store with a notification when the template changes
	*/

	return nil
}

/* remove a template and any resources from the manager */
func (r *TemplateResources) RemoveTemplate(path string) error {
	glog.V(VERBOSE_LEVEL).Infof("Removing the template resource: %s from manager", path )
	if !r.IsTemplate(path) {
		glog.Errorf("The template: %s does not exists in the manager", path )
		return errors.New("The template resource does not exist")
	}

	return nil
}
