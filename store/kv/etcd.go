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

package kv

import (
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

type EtcdStoreClient struct {
	uri string
	/* a list of etcd hosts */
	hosts []string
	/* the etcd client - under the hood is http client which should be pooled i believe */
	client *etcd.Client
}

func NewEtcdStoreClient(location *url.URL) (KVStore, error) {
	glog.Infof("Creating a Etcd Agent for K/V Store, url: %s", location)
	if location.Scheme != "etcd" {
		glog.Errorf("Invalid url: %s, must start with etcd", location)
		return nil, InvalidUrlErr
	}
	store := new(EtcdStoreClient)
	store.hosts = make([]string, 0)
	store.uri = location.String()
	for _, host := range strings.Split(location.Host, ",") {
		store.hosts = append(store.hosts, "http://"+host)
	}
	glog.Infof("Creating a Etcd Client, hosts: %s", store.hosts)
	store.client = etcd.NewClient(store.hosts)
	store.client.SetConsistency(etcd.WEAK_CONSISTENCY)
	return store, nil
}

func (r *EtcdStoreClient) URL() string {
	return r.uri
}

func (r *EtcdStoreClient) ValidateKey(key string) string {
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	return key
}

func (r *EtcdStoreClient) Get(key string) (*Node, error) {
	lookup := r.ValidateKey(key)
	/* step: lets check the cache */
	if response, err := r.GetRaw(lookup); err != nil {
		glog.Errorf("Failed to get the key: %s, error: %s", lookup, err)
		return nil, err
	} else {
		return r.CreateNode(response.Node), nil
	}
}

func (r *EtcdStoreClient) GetRaw(key string) (*etcd.Response, error) {
	glog.V(VERBOSE_LEVEL).Infof("GetRaw() key: %s", key)
	response, err := r.client.Get(key, false, true)
	if err != nil {
		glog.Errorf("Failed to get the key: %s, error: %s", key, err)
		return nil, err
	}
	return response, nil
}

func (r *EtcdStoreClient) Set(key string, value string) error {
	glog.V(VERBOSE_LEVEL).Infof("Set() key: %s, value: %s", key, value)
	_, err := r.client.Set(key, value, uint64(0))
	if err != nil {
		glog.Errorf("Failed to set the key: %s, error: %s", key, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) Delete(key string) error {
	glog.V(VERBOSE_LEVEL).Infof("Delete() deleting the key: %s", key)
	if _, err := r.client.Delete(key, false); err != nil {
		glog.Errorf("Delete() failed to delete key: %s, error: %s", key, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) RemovePath(path string) error {
	glog.V(VERBOSE_LEVEL).Infof("RemovePath() deleting the path: %s", path)
	if _, err := r.client.Delete(path, true); err != nil {
		glog.Errorf("RemovePath() failed to delete key: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) Mkdir(path string) error {
	glog.V(VERBOSE_LEVEL).Infof("Mkdir() path: %s", path)
	if _, err := r.client.CreateDir(path, uint64(0)); err != nil {
		glog.Errorf("Mkdir() failed to create directory node: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) List(path string) ([]*Node, error) {
	if !strings.HasPrefix(path, "/") || path == "" {
		path = "/" + path
	}
	glog.V(VERBOSE_LEVEL).Infof("List() path: %s", path)

	if response, err := r.GetRaw(path); err != nil {
		glog.Errorf("List() failed to get path: %s, error: %s", path, err)
		return nil, err
	} else {
		glog.V(VERBOSE_LEVEL).Infof("List() path: %s, response: %v", path, response)

		list := make([]*Node, 0)
		if response.Node.Dir == false {
			glog.Errorf("List() path: %s is not a directory node", path)
			return nil, InvalidDirectoryErr
		}
		for _, item := range response.Node.Nodes {
			list = append(list, r.CreateNode(item))
		}
		glog.V(5).Infof("List() path: %s, nodes: %v", list)
		return list, nil
	}
}

func (r *EtcdStoreClient) Watch(key string, updateChannel NodeUpdateChannel) (chan bool, error) {
	glog.V(VERBOSE_LEVEL).Infof("Watch() key: %s, channel: %V", key, updateChannel)
	stopChannel := make(chan bool)
	go func() {
		killOffWatch := false
		go func() {
			/* step: wait for the shutdown signal */
			<-stopChannel
			glog.V(VERBOSE_LEVEL).Infof("Watch() killing off the watch on key: %s", key)
			killOffWatch = true
		}()
		for {
			if killOffWatch {
				glog.V(VERBOSE_LEVEL).Infof("Watch() exitting the watch on key: %s", key)
				break
			}
			response, err := r.client.Watch(key, uint64(0), true, nil, stopChannel)
			if err != nil {
				glog.Errorf("Watch() error attempting to watch the key: %s, error: %s", key, err)
				time.Sleep(3 * time.Second)
				continue
			}
			if killOffWatch {
				continue
			}
			/* step: pass the change upstream */
			glog.V(VERBOSE_LEVEL).Infof("Watch() sending the change for key: %s upstream, event: %v", key, response)
			updateChannel <- r.GetNodeEvent(response)
		}
	}()
	return stopChannel, nil
}

func (r *EtcdStoreClient) CreateNode(response *etcd.Node) *Node {
	node := &Node{}
	node.Path = response.Key
	if response.Dir == false {
		node.Directory = false
		node.Value = response.Value
	} else {
		node.Directory = true
	}
	return node
}

func (r *EtcdStoreClient) GetNodeEvent(response *etcd.Response) (event NodeChange) {
	event.Node.Path = response.Node.Key
	event.Node.Value = response.Node.Value
	event.Node.Directory = response.Node.Dir
	switch response.Action {
	case "set":
		event.Operation = CHANGED
	case "delete":
		event.Operation = DELETED
	}
	glog.V(VERBOSE_LEVEL).Infof("GetNodeEvent() event: %s", event)
	return
}
