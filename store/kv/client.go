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
	"fmt"

	"github.com/golang/glog"
)

const STORE_VERBOSE_LEVEL = 6

var InvalidUrlErr = errors.New("Invalid URI error, please check backend url")
var InvalidDirectoryErr = errors.New("Invalid directory specified")

func Verbose(message string, args ...interface{}) {
	glog.V(STORE_VERBOSE_LEVEL).Infof(message, args)
}

type NodeUpdateChannel chan NodeChange

type KVStore interface {
	/* retrieve a key from the store */
	Get(key string) (*Node, error)
	/* Get a list of all the nodes under the path */
	List(path string) ([]*Node, error)
	/* set a key in the store */
	Set(key string, value string) error
	/* delete a key from the store */
	Delete(key string) error
	/* recursively delete a path */
	RemovePath(path string) error
	/* Create a directory node */
	Mkdir(path string) error
	/* watch for changes on the key */
	Watch(key string, updateChannel NodeUpdateChannel) (chan bool, error)
}

type Action int

const (
	UNKNOWN = 0
	CHANGED = 1
	DELETED = 2
)

type NodeChange struct {
	/* The node in question */
	Node Node
	/* The event which has occurred */
	Operation Action
}

type Node struct {
	/* the path for this key */
	Path string
	/* the value of the key */
	Value string
	/* the type of node it is, directory or file */
	Directory bool
}

func (n Node) String() string {
	return fmt.Sprintf("path: %s, value: %s, type: %s", n.Path, n.Value, n.Directory)
}

func (n Node) IsDir() bool {
	return n.Directory
}

func (n Node) IsFile() bool {
	if n.Directory {
		return false
	}
	return true
}
