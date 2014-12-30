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
	/* a list of etcd hosts */
	Hosts []string
	/* the etcd client - under the hood is http client which should be pooled i believe */
	Client *etcd.Client
}

func NewEtcdStoreClient(uri *url.URL) (KVStore, error) {
	glog.Infof("Creating a Etcd Agent for K/V Store, url: %s", uri)
	if uri.Scheme != "etcd" {
		glog.Errorf("Invalid url: %s, must start with etcd", uri)
		return nil, InvalidUrlErr
	}
	store := new(EtcdStoreClient)
	store.Hosts = make([]string, 0)
	for _, etcd_host := range strings.Split(uri.Host, ",") {
		store.Hosts = append(store.Hosts, "http://"+etcd_host)
	}
	glog.Infof("Creating a Etcd Client, hosts: %s", store.Hosts)
	store.Client = etcd.NewClient(store.Hosts)
	store.Client.SetConsistency(etcd.WEAK_CONSISTENCY)
	return store, nil
}

func (r *EtcdStoreClient) Get(key string) (*Node, error) {
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	/* step: lets check the cache */
	if response, err := r.GetRaw(key); err != nil {
		glog.Errorf("Failed to get the key: %s, error: %s", key, err)
		return nil, err
	} else {
		return r.CreateNode(response.Node), nil
	}
}

func (r *EtcdStoreClient) GetRaw(key string) (*etcd.Response,error) {
	Verbose("GetRaw() key: %s", key)
	response, err := r.Client.Get(key, false, true )
	if err != nil {
		glog.Errorf("Failed to get the key: %s, error: %s", key, err)
		return nil, err
	}
	return response, nil
}

func (r *EtcdStoreClient) Set(key string, value string) error {
	Verbose("Set() key: %s, value: %s", key, value)
	_, err := r.Client.Set(key, value, uint64(0))
	if err != nil {
		glog.Errorf("Failed to set the key: %s, error: %s", key, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) Delete(key string) error {
	Verbose("Delete() deleting the key: %s", key)
	if _, err := r.Client.Delete(key, false); err != nil {
		glog.Errorf("Delete() failed to delete key: %s, error: %s", key, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) RemovePath(path string) error {
	Verbose("RemovePath() deleting the path: %s", path)
	if _, err := r.Client.Delete(path, true); err != nil {
		glog.Errorf("RemovePath() failed to delete key: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) Mkdir(path string) error {
	Verbose("Mkdir() path: %s", path)
	if _, err := r.Client.CreateDir(path, uint64(0)); err != nil {
		glog.Errorf("Mkdir() failed to create directory node: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *EtcdStoreClient) List(path string) ([]*Node, error) {
	if !strings.HasPrefix(path, "/") || path == "" {
		path = "/" + path
	}
	Verbose("List() path: %s", path)

	if response, err := r.GetRaw(path); err != nil {
		glog.Errorf("List() failed to get path: %s, error: %s", path, err)
		return nil, err
	} else {
		glog.V(5).Infof("List() path: %s, response: %v", path, response)

		list := make([]*Node, 0)
		if response.Node.Dir == false {
			glog.Errorf("List() path: %s is not a directory node", path)
			return nil, InvalidDirectoryErr
		}
		for _, item := range response.Node.Nodes {
			list = append(list, r.CreateNode(item))
		}
		glog.V(5).Infof("List() path: %s, nodes: %v", list )
		return list, nil
	}
}

func (r *EtcdStoreClient) Watch(key string, updateChannel NodeUpdateChannel) (chan bool, error) {
	Verbose("Watch() key: %s, channel: %V", key, updateChannel)
	stopChannel := make(chan bool)
	go func() {
		killOffWatch := false
		go func() {
			/* step: wait for the shutdown signal */
			<-stopChannel
			glog.V(3).Infof("Watch() killing off the watch on key: %s", key)
			killOffWatch = true
		}()
		for {
			if killOffWatch {
				glog.V(3).Infof("Watch() exitting the watch on key: %s", key)
				break
			}
			response, err := r.Client.Watch(key, uint64(0), true, nil, stopChannel)
			if err != nil {
				glog.Errorf("Watch() error attempting to watch the key: %s, error: %s", key, err)
				time.Sleep(3 * time.Second)
				continue
			}
			if killOffWatch {
				continue
			}
			/* step: pass the change upstream */
			Verbose("Watch() sending the change for key: %s upstream", key)
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
	Verbose("GetNode() event: %s", event)
	return
}
