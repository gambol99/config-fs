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

    "github.com/gambol99/config-fs/store/kv"
    "github.com/golang/glog"

)

/* --- command line options ---- */
var (
    kv_store_url, mount_point *string
)

func init() {
    kv_store_url  = flag.String("--store", DEFAULT_KV_STORE, "the url for key / value store" )
    mount_point   = flag.String("--mount", DEFAULT_MOUNT_POINT, "the mount point for the K/V store" )
}

type Store interface {
    /* perform synchronization between the mount point and the kv store */
    Synchronize() (chan bool,error)
    /* shutdown the resources */
    Close()
}

type ConfigurationStore struct {
    /* the k/v agent for the store */
    KV          kv.KVStore
    /* fs implementation */
    FileFS      StoreFileSystem
    /* the shutdown signal */
    Shutdown    chan bool
}

func (r *ConfigurationStore) Synchronize() (chan bool,error) {
    /* step: create a shutdown signal for us */
    shutdownSignal := make(chan bool,0)
    glog.Infof("Synchronize() starting the sychronization between store and config-fs" )






    return shutdownSignal, nil
}

func (r *ConfigurationStore) FullPath(path string) string {
    return *mount_point + path
}

func (r *ConfigurationStore) BuildFileSystem() error {
    glog.Infof("Building the file system from k/v stote at: %s", *mount_point )



    return nil
}

func (r *ConfigurationStore) BuildDirectory(directory string) error {
    /* step: we get a listing of the files under the directory */
    listing, err := r.KV.List( directory )
    if err != nil {
        glog.Errorf("Failed to get listing from directory: %s, error: %s", directory, err )
        return err
    } else {
        Verbose("BuildDiectory() processing directory: %s", directory )
        for _, node := range listing {
            path := r.FullPath( directory )
            switch {
            case node.IsFile():
                /* step: if the file does not exist, create it */
                if r.FileFS.Exists(path) == false {
                    if err := r.FileFS.Create(path, node.Value); err != nil {
                        glog.Errorf("Failed to create the file: %s, error: %s", path, err)
                        continue
                    }
                }
            case node.IsDir():
                if r.FileFS.Exists(path) == true && r.FileFS.IsDirectory(path) == true {
                    Verbose("BuildDiectory() item directory: %s already exists, skipping", path)
                    continue
                }
                if !r.FileFS.Exists(path) {
                    Verbose("BuildDiectory() creating directory item: %s", path)
                    r.FileFS.Mkdir(path)
                }
                /* go recursive and build the contents of that directory */
                go func() {
                    if err := r.BuildDirectory(path); err != nil {
                        glog.Errorf("Failed to build the item directory: %s, error: %s", path, err)
                    }
                }()
            }
        }
    }
    return nil
}


func (r *ConfigurationStore) Close() {
    glog.Infof("Request to shutdown and release the resources" )
    r.Shutdown <- true
}
