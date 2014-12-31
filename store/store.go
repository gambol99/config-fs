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
	"fmt"
	"time"

	"github.com/gambol99/config-fs/store/discovery"
	"github.com/gambol99/config-fs/store/kv"
	"github.com/golang/glog"
)

/* --- command line options ---- */
var (
	kv_store_url, mount_point *string
	delete_on_exit, read_only *bool
	refresh_interval          *int

)

func init() {
	kv_store_url = flag.String("store", DEFAULT_KV_STORE, "the url for key / value store")
	mount_point = flag.String("mount", DEFAULT_MOUNT_POINT, "the mount point for the K/V store")
	delete_on_exit = flag.Bool("delete", DEFAULT_DELETE_ON_EXIT, "delete all configuration on exit")
	refresh_interval = flag.Int("interval", DEFAULT_INTERVAL, "the default interval for performed a forced resync")
	read_only = flag.Bool("readonly", DEFAULT_READ_ONLY, "wheather or not the config store of read-only")
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

func (r *ConfigurationStore) Close() {
	glog.Infof("Request to shutdown and release the resources")
	r.Shutdown <- true
}

/* Synchronize the key/value store with the configuration directory */
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
			case event := <-TemplateChannel:
				go r.HandleTemplateEvent(event)
			case <-TimerChannel.C:
				go r.HandleTimerEvent()
			case <-r.Shutdown:
				glog.Infof("Synchronize() recieved the shutdown signal :-( ... shutting down")
				break
			}
		}
	}()
	return nil
}

/* ============== EVENT HANDLING ================= */

/* Handle a change to the templated resource */
func (r *ConfigurationStore) HandleTemplateEvent(event TemplateUpdateEvent) {
	glog.V(VERBOSE_LEVEL).Infof("HandleTemplateEvent() recieved node event: %s, resynchronizing", event)
}

/* We have a timer event, let force re-sync the configuration */
func (r *ConfigurationStore) HandleTimerEvent() {
	glog.V(VERBOSE_LEVEL).Infof("HandleTimerEvent() recieved ticker event , kicking off a synchronization")

}

/* Handle changes to the K/V store and reflect in the directory */
func (r *ConfigurationStore) HandleNodeEvent(event kv.NodeChange) {
	glog.V(VERBOSE_LEVEL).Infof("HandleNodeEvent() recieved node event: %s, synchronizing", event)
	node := event.Node
	/* check: an update or deletion */
	switch event.Operation {
	case kv.DELETED:
		if node.IsDir() {
			r.DeleteStoreConfigDirectory(node.Path)
		} else if node.IsFile() {
			r.DeleteStoreConfigFile(node.Path)
		}
	case kv.CHANGED:
		r.ProcessNodeUpdate(event.Node)
	default:
		glog.Errorf("HandleNodeEvent() unknown operation, skipping the event: %s", event)
	}
}

/* ============= K/V Updates and changes ================ */

/* Delete a file from the config store */
func (r *ConfigurationStore) DeleteStoreConfigFile(path string) error {
	/* the actual file system path */
	full_path := r.FullPath(path)
	glog.V(VERBOSE_LEVEL).Infof("DeleteStoreConfigFile() Deleting configuration file: %s from the store", full_path )
	/* step: check it exists and is a file */
	if !r.Exists(full_path) || !r.IsFile(full_path) {
		glog.Errorf("Failed to delete file: %s, either it doesnt exists or is not a file", full_path )
		return errors.New("Failed to delete, either it doesnt exists or is not a file")
	}
	/* check: is the file a templated resource */
	if r.Resources.IsTemplate(path) {
		glog.V(VERBOSE_LEVEL).Infof("Deleting the templated resource: %s", full_path )
		/* step: free up the resources from the resource manager */
		r.Resources.RemoveTemplate(path)
		/* step: delete the actual file */
		if err := r.Delete(full_path); err != nil {
			glog.Errorf("Failed to delete the file: %s, error: %s", full_path, err )
			return err
		}
	}
	/* step: delete the file */
	if err := r.Delete(full_path); err != nil {
		glog.Errorf("Failed to delete the file: %s, error: %s", full_path, err )
		return err
	}
	return nil
}

func (r *ConfigurationStore) DeleteStoreConfigDirectory(path string) error {
	/* the actual file system path */
	full_path := r.FullPath(path)
	glog.V(VERBOSE_LEVEL).Infof("DeleteStoreConfigDirectory() Deleting configuration directory: %s from the store", full_path )
	/* step: check it is a actual directory */
	if _, err := r.CheckDirectory(full_path); err != nil {
		glog.Errorf("Failed to remove the directory: %s, error: %s", full_path, err)
		return err
	}
	/* @TODO step: we need to remove any watches on the filesystem */

	/* @TODO step: we need to remove any templated resources which were in the directory */


	/* step: delete the directory and all the children */
	if err := r.RmDir(full_path); err != nil {
		glog.Errorf("Failed to delete the directory: %s, error: %s", full_path, err )
		return err
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

/* ======= MISC ========== */

/* Converts the k/v path to the full path on disk - essentially mount_point + node_path */
func (r *ConfigurationStore) FullPath(path string) string {
	return fmt.Sprintf("%s%s", *mount_point, path )
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
				glog.V(VERBOSE_LEVEL).Infof("BuildDirectory() Creating the file: %s", path)
				if err := r.Create(path, node.Value); err != nil {
					glog.Errorf("Failed to create the file: %s, error: %s", path, err)
				}
			case node.IsDir():
				if r.Exists(path) == false {
					glog.V(VERBOSE_LEVEL).Infof("BuildDiectory() creating directory item: %s", path)
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

