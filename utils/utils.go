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

package utils

import (
	"errors"
	"time"

	"github.com/golang/glog"
)

func Attempt(method func() (interface{}, error), attempts, interval int) (interface{}, error) {
	for i := 0; i < attempts; i++ {
		result, err := method()
		if err != nil {
			/* step: lets sleep for x seconds and try again */
			time.Sleep(time.Duration(interval) * time.Second)
		} else {
			return result, nil
		}
	}
	return nil, errors.New("Failed to implement the method")
}

func Forever(method func() error) error {
	go func() {
		for {
			if err := method(); err != nil {
				glog.Errorf("Method failed to execute, error: %s, restarting now", err)
			}
		}
	}()
	return nil
}
