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
	"flag"
	"net/url"
	"time"

	"github.com/gambol99/config-fs/store/discovery"
	"github.com/gambol99/config-fs/store/kv"
	"github.com/golang/glog"
)

/* --- command line options ---- */
var (
	kv_store_url, mount_point *string
	delete_on_exit            *bool
	refresh_interval          *int
)

func init() {
	kv_store_url = flag.String("store", DEFAULT_KV_STORE, "the url for key / value store")
	mount_point = flag.String("mount", DEFAULT_MOUNT_POINT, "the mount point for the K/V store")
	delete_on_exit = flag.Bool("delete", DEFAULT_DELETE_ON_EXIT, "delete all configuration on exit")
	refresh_interval = flag.Int("interval", DEFAULT_INTERVAL, "the default interval for performed a forced resync")
}

/*
The interface to the config-fs
*/
type Store interface {
	/* perform synchronization between the mount point and the kv store */
	Synchronize() error
	/* shutdown the resources */
	Close()
}

/*
The implementation of the above
*/
type ConfigurationStore struct {
	StoreFileSystem
	/* the k/v agent for the store */
	KV kv.KVStore
	/* discovery agent */
	Agent discovery.Discovery
	/* the template manager */
	Resources TemplateManager
	/* the shutdown signal */
	Shutdown chan bool
}

/*
Create a new configuration store
*/
func NewStore() (Store, error) {
	glog.Infof("Creating a new configuration store, mountpoint: %s, kv: %s", *mount_point, *kv_store_url)
	store := &ConfigurationStore{NewStoreFS(), nil, nil, nil, nil}
	/* the shutdown signal for the store */
	store.Shutdown = make(chan bool, 1)
	/* the discovery manager */
	store.Agent = discovery.NewDiscoveryService()
	/* the resource manager */
	store.Resources = NewTemplateManager(store.Agent)
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

/* converts the k/v path to the full path on disk - essentially mount_point + node_path */
func (r *ConfigurationStore) FullPath(path string) string {
	return *mount_point + path
}

/*
Synchronize the key/value store with the configuration directory
*/
func (r *ConfigurationStore) Synchronize() error {
	/* step: start by performing a initial config build */
	glog.Infof("Synchronize() starting the sychronization between mount: %s and store: %s", *mount_point, *kv_store_url)
	if err := r.BuildFileSystem(); err != nil {
		glog.Errorf("Failed to build the initial filesystem, error: %s", err)
		return err
	}

	/* step: jump into the event loop; we wait for
	- a change to occur in the K/V store
	- a timer event to occur and enforce a refresh of the config
	- a notification of file changes on the config directory
	- a template resource has changed and we need to update the config store
	- a shutdown signal to occur
	*/
	go func() {
		/* step: create the timer */
		TimerChannel := time.NewTicker(time.Duration(*refresh_interval) * time.Second)
		/* step: create the watch on the base */
		NodeChannel := make(kv.NodeUpdateChannel, 1)
		/* step: create a channel for sending updates on template resources */
		TemplateChannel := make(TemplateUpdateChannel, 0)

		/* step: create a watch of the K/V */
		if _, err := r.KV.Watch("/", NodeChannel); err != nil {
			glog.Errorf("Failed to add watch to root directory, error: %s", err)
			return
		}
		/* step: never say die - unless asked to of course */
		for {
			select {
			case event := <-NodeChannel:
				go r.HandleNodeEvent(event)
			case <-TimerChannel.C:
				go r.HandleTimerEvent()
			case event := <-TemplateChannel:
				go r.HandleTemplateEvent(event)
			case <-r.Shutdown:
				glog.Infof("Synchronize() recieved the shutdown signal :-( ... shutting down")
				break
			}
		}
	}()
	return nil
}

/* Handle a change to the templated resource */
func (r *ConfigurationStore) HandleTemplateEvent(event TemplateUpdateEvent) {

}

/*
We have a timer event, let force re-sync the configuration
*/
func (r *ConfigurationStore) HandleTimerEvent() {
	glog.V(5).Infof("ProcessTimerEvent() recieved ticker event , kicking off a synchronization")

}

/*
Handle changes to the K/V store and reflect in the directory
*/
func (r *ConfigurationStore) HandleNodeEvent(event kv.NodeChange) {
	glog.V(5).Infof("ProcessNodeEvent() recieved node event: %s, resynchronizing", event)
	/* check: an update or deletion */
	switch event.Operation {
	case kv.DELETED:
		r.ProcessNodeDeletion(event.Node)
	case kv.CHANGED:
		r.ProcessNodeUpdate(event.Node)
	default:
		glog.Errorf("ProcessTimerEvent() unknown operation, skipping the event: %s", event)
	}
}

func (r *ConfigurationStore) ProcessNodeDeletion(node kv.Node) error {
	fullPath := r.FullPath(node.Path)
	glog.V(VERBOSE_LEVEL).Infof("ProcessNodeDeletion() node: %s, path: %s", node, fullPath)
	switch {
	case node.IsDir():
		glog.V(VERBOSE_LEVEL).Infof("Deleting the directory: %s", fullPath)
		/* step: check it is a actual directory */
		if _, err := r.CheckDirectory(fullPath); err != nil {
			glog.Errorf("Failed to remove the directory: %s, error: %s", fullPath, err)
			return err
		}
		/* step: attempt to remove it */
		if err := r.Rmdir(fullPath); err != nil {
			glog.Errorf("Failed to remove the directory: %s, error: %s", fullPath, err)
		}
	case node.IsFile():
		glog.V(VERBOSE_LEVEL).Infof("Deleting the file: %s", fullPath)
		if r.Exists(fullPath) && r.IsFile(fullPath) {
			if err := r.Delete(fullPath); err != nil {
				glog.Errorf("Failed to delete the file: %s, error: %s", fullPath, err)
				return err
			}
			glog.V(VERBOSE_LEVEL).Infof("Successfully deleted the file: %s", fullPath)
		} else {
			glog.Errorf("Failed to delete the file: %s, the file doesnt exist or is not a file", fullPath)
			return errors.New("Failed to delete the file, either not a file or does not exist")
		}
	default:
		return errors.New("Unknown node type, neither file or directory")
	}
	return nil
}

/* A node has been change, altered or created */
func (r *ConfigurationStore) ProcessNodeUpdate(node kv.Node) error {
	fullPath := r.FullPath(node.Path)
	glog.V(VERBOSE_LEVEL).Infof("ProcessNodeUpdate() node: %s, path: %s", node, fullPath)
	parentDirectory := r.Dirname(fullPath)
	switch {
	case node.IsDir():
		/* step: we need to make sure the directory structure exists */
		if err := r.Mkdirp(parentDirectory); err != nil {
			glog.Errorf("Failed to ensure the directory: %s, error: %s", parentDirectory, err)
			return err
		}
	case node.IsFile():
		if err := r.Mkdirp(parentDirectory); err != nil {
			glog.Errorf("Failed to ensure the directory: %s, error: %s", parentDirectory, err)
			return err
		}
		/* step: create the file */
		if err := r.Create(fullPath, node.Value); err != nil {
			glog.Errorf("Failed to create the file: %s, error: %s", fullPath, err)
			return err
		}
	}
	return nil
}

func (r *ConfigurationStore) CheckDirectory(path string) (bool, error) {
	if r.Exists(path) == false {
		return false, DirectoryDoesNotExistErr
	}
	if r.IsDirectory(path) == false {
		return false, IsNotDirectoryErr
	}
	return true, nil
}

func (r *ConfigurationStore) BuildFileSystem() error {
	glog.Infof("Building the file system from k/v stote at: %s", *mount_point)
	r.BuildDirectory("/")
	return nil
}

func (r *ConfigurationStore) BuildDirectory(directory string) error {
	/* step: we get a listing of the files under the directory */
	listing, err := r.KV.List(directory)
	if err != nil {
		glog.Errorf("Failed to get listing from directory: %s, error: %s", directory, err)
		return err
	} else {
		Verbose("BuildDiectory() processing directory: %s", directory)
		for _, node := range listing {
			path := r.FullPath(node.Path)
			glog.V(5).Infof("BuildDirectory() directory: %s, full path: %s", directory, path)
			switch {
			case node.IsFile():
				/* step: if the file does not exist, create it */
				Verbose("BuildDirectory() Creating the file: %s", path)
				if err := r.Create(path, node.Value); err != nil {
					glog.Errorf("Failed to create the file: %s, error: %s", path, err)
				}
			case node.IsDir():
				if r.Exists(path) == false {
					Verbose("BuildDiectory() creating directory item: %s", path)
					r.Mkdir(path)
				}
				/* go recursive and build the contents of that directory */
				if err := r.BuildDirectory(node.Path); err != nil {
					glog.Errorf("Failed to build the item directory: %s, error: %s", path, err)
				}
			}
		}
	}
	return nil
}

func (r *ConfigurationStore) Close() {
	glog.Infof("Request to shutdown and release the resources")
	r.Shutdown <- true
}
