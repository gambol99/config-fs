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
	"flag"
	"time"

	"github.com/gambol99/config-fs/store/kv"
	"github.com/gambol99/config-fs/store/discovery"
	"github.com/golang/glog"
)

/* --- command line options ---- */
var (
	kv_store_url, mount_point *string
	delete_on_exit *bool
	refresh_interval *int
)

func init() {
	kv_store_url     = flag.String("store", DEFAULT_KV_STORE, "the url for key / value store")
	mount_point      = flag.String("mount", DEFAULT_MOUNT_POINT, "the mount point for the K/V store")
	delete_on_exit 	 = flag.Bool("delete", DEFAULT_DELETE_ON_EXIT, "delete all configuration on exit" )
	refresh_interval = flag.Int("interval", DEFAULT_INTERVAL, "the default interval for performed a forced resync" )
}

type Store interface {
	/* perform synchronization between the mount point and the kv store */
	Synchronize() error
	/* shutdown the resources */
	Close()
}

type ConfigurationStore struct {
	/* the k/v agent for the store */
	KV kv.KVStore
	/* fs implementation */
	FileFS StoreFileSystem
	/* discovery agent */
	Agent  discovery.Discovery
	/* the shutdown signal */
	Shutdown chan bool
}

func (r *ConfigurationStore) Synchronize() error {
	/* step: create a shutdown signal for us */
	glog.Infof("Synchronize() starting the sychronization between mount: %s and store: %s", *mount_point, *kv_store_url )
	if err := r.BuildFileSystem(); err != nil {
		glog.Errorf("Failed to build the initial filesystem, error: %s", err )
		return err
	}
	/* step: we wait for a timer, a node update or a shutdown signal */
	go func() {
		/* step: create the timer */
		TimerChannel := time.NewTicker(time.Duration(*refresh_interval) * time.Second )
		/* step: create the watch on the base */
		NodeChannel  := make(kv.NodeUpdateChannel,1)
		/* step: create a watch of the K/V */
		if  _, err := r.KV.Watch("/", NodeChannel ); err != nil {
			glog.Errorf("Failed to add watch to root directory, error: %s", err )
			return
		}
		/* step: never say die - unless asked to of course */
		for {
			select {
			case event := <- NodeChannel:
				glog.V(5).Infof("Synchronize() recieved node event: %s, resynchronizing", event )
				if event.Node.IsFile() {
					if err := r.FileFS.Create(r.FullPath( event.Node.Path ), event.Node.Value); err != nil {
						glog.Errorf("Failed to create the file: %s, error: %s", event.Node.Path, err)
					}
				} else {
					go r.BuildDirectory( event.Node.Path )
				}
			case <- TimerChannel.C:
				glog.V(5).Infof("Synchronize() recieved ticker event , kicking off a synchronization" )
				go r.BuildFileSystem()
			case <- r.Shutdown:
				glog.Infof("Synchronize() recieved the shutdown signal :-( ... shutting down" )
				break
			}
		}
	}()
	return nil
}

func (r *ConfigurationStore) FullPath(path string) string {
	return *mount_point + path
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
			path := r.FullPath( node.Path )
			glog.V(5).Infof("BuildDirectory() directory: %s, full path: %s", directory, path )
			switch {
			case node.IsFile():
				/* step: if the file does not exist, create it */
				Verbose("BuildDirectory() Creating the file: %s", path )
				if err := r.FileFS.Create(path, node.Value); err != nil {
					glog.Errorf("Failed to create the file: %s, error: %s", path, err)
				}
			case node.IsDir():
				if r.FileFS.Exists(path) == false {
					Verbose("BuildDiectory() creating directory item: %s", path)
					r.FileFS.Mkdir(path)
				}
				/* go recursive and build the contents of that directory */
				if err := r.BuildDirectory( node.Path ); err != nil {
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
