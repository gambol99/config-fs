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
	"errors"
	"net/url"

	"github.com/gambol99/config-fs/store/kv"
	"github.com/golang/glog"
)

func NewStore() (Store, error) {
	glog.Infof("Creating a new configuration store, mountpoint: %s, kv: %s", *mount_point, *kv_store_url)
	store := new(ConfigurationStore)
	store.Shutdown = make(chan bool, 1)
	store.FileFS = NewStoreFS()
	/* step: parse the url */
	uri, err := url.Parse(*kv_store_url)
	if err != nil {
		glog.Errorf("Failed to parse thr k/v url: %s, error: %s", *kv_store_url, err)
		return nil, err
	}
	/* step: get the correct the kv agent */
	switch uri.Scheme {
	case "etcd":
		if agent, err := kv.NewEtcdStoreClient(uri); err != nil {
			glog.Errorf("Failed to create the K/V agent, error: %s", err)
			return nil, err
		} else {
			store.KV = agent
		}
	default:
		return nil, errors.New("Unsupported key/value store: " + *kv_store_url)
	}
	return store, nil
}
