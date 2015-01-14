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
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

type EtcdStoreClient struct {
	/* a lock for the watcher map */
	sync.RWMutex
	/* the base root key */
	baseKey string
	/* the url of the etcd hosts */
	uri string
	/* a list of etcd hosts */
	hosts []string
	/* the etcd client - under the hood is http client which should be pooled i believe */
	client *etcd.Client
	/* stop channel for the client */
	stopChannel chan bool
	/* stop channel for reciever */
	stopRecieverChannel chan bool
	/* the update channel we send our changes to */
	channel NodeUpdateChannel
	/* a map of keys presently being watched */
	watchedKeys map[string]bool
}

func NewEtcdStoreClient(location *url.URL, channel NodeUpdateChannel) (KVStore, error) {
	/* step: create the client */
	store := new(EtcdStoreClient)
	store.baseKey = "/"
	store.hosts = make([]string, 0)
	store.uri = location.String()
	store.channel = channel
	store.watchedKeys = make(map[string]bool, 0)
	store.stopChannel = make(chan bool)

	for _, host := range strings.Split(location.Host, ",") {
		store.hosts = append(store.hosts, "http://"+host)
	}
	glog.Infof("Creating a Etcd Agent for K/V Store, host: %s", store.hosts)

	/* step: create the etcd client */
	store.client = etcd.NewClient(store.hosts)
	store.client.SetConsistency(etcd.WEAK_CONSISTENCY)

	/* step: start watching for events */
	store.WatchEvents()

	return store, nil
}

func (r *EtcdStoreClient) Close() {
	glog.Infof("Shutting down the etcd client")
	r.stopChannel <- true
}

func (r *EtcdStoreClient) WatchEvents() {
	glog.V(VERBOSE_LEVEL).Infof("Starting the event watcher for the etcd clinet, channel: %v", r.channel)
	/* the kill switch for the goroutine */
	kill_off := false

	/* routine: waits on the shutdown signal for the client and flicks the kill switch */
	go func() {
		glog.V(VERBOSE_LEVEL).Infof("Waiting on a shutdown signal from consumer, channel: %v", r.channel)
		/* step: wait for the shutdown signal */
		<-r.stopChannel
		/* @perhaps : we could speed up the take down by using a stop channel on the watch? */
		glog.V(VERBOSE_LEVEL).Infof("Flicking the kill switch for watcher, channel: %v", r.baseKey, r.channel)
		kill_off = true
	}()

	/* routine: loops around watching until flick the switch */
	go func() {
		/* step: set the index to zero for now */
		wait_index := uint64(0)
		/* step: look until we hit the kill switch */
		for {
			if kill_off {
				break
			}
			/* step: apply a watch on the key and wait */
			response, err := r.client.Watch(r.baseKey, wait_index, true, nil, nil)
			if err != nil {
				glog.Errorf("Failed to attempting to watch the key: %s, error: %s", r.baseKey, err)
				time.Sleep(3 * time.Second)
				wait_index = uint64(0)
				continue
			}
			/* step: have we been requested to quit */
			if kill_off {
				continue
			}
			/* step: update the wait index */
			wait_index = response.Node.ModifiedIndex + 1

			/* step: cool - we have a notification - lets check if this key is being watched */
			go r.ProcessNodeChange(response)
		}
		glog.V(VERBOSE_LEVEL).Infof("Exitted the k/v watcher routine, channel: %v", r.channel)
	}()
}

func (r *EtcdStoreClient) URL() string {
	return r.uri
}

func (r *EtcdStoreClient) ValidateKey(key string) string {
	/* step: if it doesnt start with a / - add it */
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	/* step: if it ends with a slash, remove it */
	if len(key) > 1 && strings.HasSuffix(key, "/")  {
		key = key[:len(key)-1]
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
	key := r.ValidateKey(path)
	glog.V(VERBOSE_LEVEL).Infof("List() path: %s", key)
	if response, err := r.GetRaw(path); err != nil {
		glog.Errorf("List() failed to get path: %s, error: %s", key, err)
		return nil, err
	} else {
		list := make([]*Node, 0)
		if response.Node.Dir == false {
			glog.Errorf("List() path: %s is not a directory node", key)
			return nil, InvalidDirectoryErr
		}
		for _, item := range response.Node.Nodes {
			list = append(list, r.CreateNode(item))
		}
		return list, nil
	}
}

func (e *EtcdStoreClient) Paths(path string, paths *[]string) ([]string, error) {
	response, err := e.client.Get(path, false, true)
	if err != nil {
		return nil, errors.New("Unable to complete walking the tree" + err.Error())
	}
	for _, node := range response.Node.Nodes {
		if node.Dir {
			e.Paths(node.Key, paths)
		} else {
			glog.Infof("Found service container: %s appending now", node.Key)
			*paths = append(*paths, node.Key)
		}
	}
	return *paths, nil
}

func (r *EtcdStoreClient) Watch(key string) {
	r.Lock()
	defer r.Unlock()
	/* step: we check if the key is being watched and if not add it */
	if _, found := r.watchedKeys[key]; found {
		glog.V(VERBOSE_LEVEL).Infof("Thy key: %s is already being wathed, skipping for now", key)
	} else {
		glog.V(VERBOSE_LEVEL).Infof("Adding a watch on the key: %s", key)
		r.watchedKeys[key] = true
	}
}

func (r *EtcdStoreClient) ProcessNodeChange(response *etcd.Response) {
	/* step: are there any keys being watched */
	if len(r.watchedKeys) <= 0 {
		return
	}
	r.RLock()
	defer r.RUnlock()
	/* step: iterate the list and find out if our key is being watched */
	path := response.Node.Key
	glog.V(VERBOSE_LEVEL).Infof("Checking if key: %s is being watched", path)
	for watch_key, _ := range r.watchedKeys {
		if strings.HasPrefix(path, watch_key) {
			glog.V(VERBOSE_LEVEL).Infof("Sending notification of change on key: %s, channel: %v, event: %v", path, r.channel, response)
			/* step: we create an event and send upstream */
			var event NodeChange
			event.Node.Path = response.Node.Key
			event.Node.Value = response.Node.Value
			event.Node.Directory = response.Node.Dir
			switch response.Action {
			case "set":
				event.Operation = CHANGED
			case "delete":
				event.Operation = DELETED
			}
			/* step: send the event upstream via the channel */
			r.channel <- event
			return
		}
	}
	glog.V(VERBOSE_LEVEL).Infof("The key: %s is presently not being watched, we can ignore for now", path )
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
