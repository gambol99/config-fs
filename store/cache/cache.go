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

package cache

import (
	"sync"
	"time"

	"github.com/golang/glog"
)

type Cache interface {
	/* retrieve an entry from the cache */
	Get(string) (interface{},bool)
	/* set a entry in the cache */
	Set(string, interface{},time.Duration)
	/* Remvoe a key from the cache */
	Delete(string)
	/* check if a key exist in the cache */
	Exists(string) bool
	/* get the size of the cache */
	Size() int
	/* flush the cache all entries */
	Flush() error
}

type CachedItem struct {
	/* the unix timestamp the item is expiring in */
	Expiring    int64
	/* the data we are holding */
	Data 		interface {}
}

type CacheStore struct {
	/* locking required for the cache */
	sync.RWMutex
	/* the cache map */
	Items map[string]*CachedItem
}

func NewCacheStore() Cache {
	glog.Infof("Creating a new cache store")
	return &CacheStore{Items: make(map[string]*CachedItem)}
}

/*
The reaper is responsible for looping around and removing any cache item thats
old
 */
func (c *CacheStore) Reaper() {
	go func() {
		for {
			/* step: reap any old items */
			c.ReapItems()
			/* step: go to sleep */
			time.Sleep( 5 * time.Second )
		}
	}()
}

func (c *CacheStore) ReapItems() {
	c.Lock()
	defer c.Unlock()
	if len( c.Items ) <= 0 {
		glog.V(5).Infof("Cache Reaper: no items in the cache to reap" )
		return
	}
	now := time.Now().Unix()
	for key, item := range c.Items {
		if item.Expiring > 0 && now >= item.Expiring {
			glog.V(6).Infof("Expiring the item: %s", item )
			delete( c.Items, key )
		}
	}
}

func (c *CacheStore) Get(key string) (interface{},bool) {
	/* step: if the key is empty return */
	if key == "" {
		return nil, false
	}
	c.RLock()
	defer c.RUnlock()
	glog.V(9).Infof("Get() key: %s", key)
	if item, found := c.Items[key]; found {
		glog.V(10).Infof("Get() key: %s found in cache", key )
		return item.Data, true
	}
	return nil, false
}

func (c *CacheStore) Set(key string, item interface{}, ttl time.Duration ) {
	/* step: if the key is empty return */
	if key == "" {
		return
	}
	c.Lock()
	defer c.Unlock()
	expiration_time := int64(0)
	if ttl != 0 {
		expiration_time = (time.Now().Unix()) + int64(ttl.Seconds())
	}
	glog.V(9).Infof("Set() key: %s, value: %V, expiration: %d", key, item, expiration_time )
	c.Items[key] = &CachedItem{expiration_time,item}
}

func (c *CacheStore) Delete(key string) {
	if found := c.Exists(key); found {
		c.Lock()
		defer c.Unlock()
		delete(c.Items, key)
	}
}

func (c *CacheStore) Exists(key string) (found bool) {
	c.RLock()
	defer c.RUnlock()
	_, found = c.Items[key]
	return
}

func (c *CacheStore) Flush() error {
	c.Lock()
	defer c.Unlock()
	c.Items = make(map[string]*CachedItem)
	return nil
}

func (c *CacheStore) Size() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.Items)
}
