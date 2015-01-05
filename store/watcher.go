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
	"sync"
	"os"
	"path/filepath"

	"github.com/go-fsnotify/fsnotify"
	"github.com/golang/glog"
)

/* the interface to the service */
type WatchService interface {
	/* watch for changes in this directory */
	AddDirectoryWatch(path string) error
	/* remove the directory watch */
	RemoveDirectoryWatch(path string) error
	/* add a listener for changes in the file system */
	AddWatchListener(listener WatchServiceChannel)
}

/* the implementation of the above */
type Watcher struct {
	/* a lock for the map above */
	sync.RWMutex
	/* a list of those listening for events */
	listeners map[WatchServiceChannel]bool
	/* the file system watcher */
	watcher *fsnotify.Watcher
	/* a map of the directories which are currently being watched */
	directories map[string]bool
}

/* the channel used to send events */
type WatchServiceChannel chan *fsnotify.Event

/* create an instance */
func NewWatchService() (WatchService,error) {
	glog.Infof("Creating a new Watch Service")
	/* step: create a watcher */
	if watcher, err := fsnotify.NewWatcher(); err != nil {
		glog.Errorf("Failed to create a watcher, error: %s", err )
		return nil, err
	} else {
		service := new(Watcher)
		service.listeners = make(map[WatchServiceChannel]bool,0)
		service.watcher = watcher
		service.directories = make(map[string]bool,0)
		return service, nil
	}
}

func (r *Watcher) AddDirectoryWatched(path string) {
	r.Lock()
	defer r.Unlock()
	r.directories[path] = true
}

/* remove the directory watch */
func (r *Watcher) RemoveDirectoryWatch(path string) error {

	return nil
}

/* add a listener for changes in the file system */
func (r *Watcher) AddWatchListener(listener WatchServiceChannel) {
	glog.V(VERBOSE_LEVEL).Infof("Adding the new event listener: %v", listener )
	r.Lock()
	defer r.Unlock()
	r.listeners[listener] = true
}

func (r *Watcher) RemoveWatchLister(listener WatchServiceChannel) {
	glog.V(VERBOSE_LEVEL).Infof("Removing the listen: %v from the list", listener )
	r.Lock()
	defer r.Unlock()
	for channel, _ := range r.listeners {
		if channel == listener {
			delete(r.listeners,channel)
			return
		}
	}
}

func (r *Watcher) AddDirectoryWatch(path string) error {
	glog.V(VERBOSE_LEVEL).Infof("Adding directory watch on directory: %s", path)
	/* step: check the directory actually exists */
	if directory, err := r.IsDirectory(path); err != nil {
		glog.Errorf("Failed to stat the directory: %s, error: %s", path, err)
		return err
	} else {
		if directory == false {
			glog.Errorf("Failed to add directory watch on: %s, directory does not exist", path)
			return DirectoryDoesNotExistErr
		}
	}

	/* check if the directory is already being watched */
	if _, found := r.directories[path]; found {
		glog.V(VERBOSE_LEVEL).Infof("The directory: %s is already being watched, skipping for now", path)
	}

	/* step: add the directory and all subdirectores to the watcher */
	if err := r.watcher.Add(path); err != nil {
		glog.Errorf("Failed to add a watcher on the directory: %s, error: %s", path, err)
		return err
	} else {
		/* step: add to the list of watcher directories */
		r.AddDirectoryWatch(path)
		/* step: we need to get a list of subdirectories */
		if paths, err := r.ListDirectories(path); err != nil {
			glog.Errorf("Failed to get a list of subdirectories from path: %s, error: %s", path, err)
			return err
		} else {
			/* step: we iterate the folder and add a watch on each directory */
			for _, subdirectory := range paths {
				if err := r.AddDirectoryWatch(subdirectory); err != nil {
					glog.Errorf("Failed to add a watch on the subdirectory: %s", subdirectory)
				}
			}
		}
	}
	return nil
}

func (r *Watcher) Exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (r *Watcher) IsDirectory(path string) (bool,error) {
	if found := r.Exists(path); !found {
		return false, nil
	}
	if stat, err := os.Stat(path); err != nil {
		glog.Errorf("Failed to stat the path: %s, error: %s", path, err)
		return false, err
	} else {
		return stat.IsDir(), nil
	}
}

func (r *Watcher) ListDirectories(path string) ([]string, error) {
	paths := make([]string, 0)
	if err := filepath.Walk(path, (filepath.WalkFunc)(func(file_path string, info os.FileInfo, err error) error {
			if err != nil {
				glog.Errorf("Failed to walk the directory: %s", file_path)
				return err
			}
			if info.IsDir() {
				name := info.Name()
				if name != "." && name != ".." {
					return filepath.SkipDir
				}
				paths = append(paths, file_path)
			}
			return nil
		})); err != nil {
		glog.Errorf("Failed to walk the directory: %s, error: %s", path, err)
		return nil, err
	}
	return paths, nil
}
