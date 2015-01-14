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

package fs

import (
	"crypto/md5"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
)

const (
	VERBOSE_LEVEL           = 6
	DEFAULT_DIRECTORY_PERMS = 0755
	DEFAULT_FILE_PERMS      = 0644
)

var (
	DoesNotExistErr          = errors.New("The request entruy does not exists")
	DirectoryDoesNotExistErr = errors.New("The directory does not exist")
	FileDoesNotExistErr      = errors.New("The file does not exist")
	NotFileErr               = errors.New("The argument is not a value file path")
	IsNotDirectoryErr        = errors.New("The path is not a directory")
)

type FileStore interface {
	/* create a file from a k/v */
	Create(path string, value string) error
	/* update the file */
	Update(path string, value string) error
	/* delete the file */
	Delete(path string) error
	/* get a list of the children */
	List(path string) ([]string, error)
	/* check if exists */
	Exists(path string) bool
	/* checks if a directory */
	IsDirectory(path string) bool
	/* checks if file */
	IsFile(path string) bool
	/* create a directory */
	Mkdir(path string) error
	/* create a directory structure */
	Mkdirp(path string) error
	/* delete the directory */
	Rmdir(path string) error
	/* get the hash of the file content */
	Hash(path string) (string, error)
	/* touch the file */
	Touch(path string) error
	/* parent directory */
	Dirname(path string) string
}

type StoreFS struct {
}

func NewStoreFS() FileStore {
	return new(StoreFS)
}

func (r *StoreFS) Create(path string, value string) error {
	parentDirectory := filepath.Dir(path)
	if !r.IsDirectory(parentDirectory) {
		glog.Errorf("Failed to create file: %s, parent: %s does not exist", path, parentDirectory)
		return DirectoryDoesNotExistErr
	}
	/* step: create the file */
	glog.V(5).Infof("Create() path: %s, creating file, value: %s", path, value)
	if fs, err := os.Create(path); err != nil {
		glog.Errorf("Failed to create the file: %s, error: %s", path, err)
		return err
	} else {
		/* change the perms to read only */
		fs.Chmod(os.FileMode(DEFAULT_FILE_PERMS))
		if _, err := fs.WriteString(value); err != nil {
			glog.Errorf("Failed to write the contents to file, error: %s", err)
			fs.Close()
			os.Remove(path)
		} else {
			fs.Close()
		}
	}
	return nil
}

func (r *StoreFS) Update(path string, value string) error {
	if !r.Exists(path) || !r.IsFile(path) {
		glog.Errorf("The file: %s does not exist or is not a file", path)
		return NotFileErr
	}

	/* step: we only update if the content is different */
	if file_sum, err := r.Hash(path); err != nil {
		glog.Errorf("Failed to generate a hash on the file: %s, error: %s", path, err)
		return err
	} else {
		/* step: get a hash of the new content */
		content_sum := r.HashString(value)
		if file_sum == content_sum {
			glog.Infof("The content of config file: %s has not changed, skipping the update", path)
		} else {
			if fs, err := os.Create(path); err != nil {
				glog.Errorf("Failed to create the file: %s, error: %s", path, err)
				return err
			} else {
				defer fs.Close()
				if _, err := fs.WriteString(value); err != nil {
					glog.Errorf("Failed to write the contents to file, error: %s", err)
				}
				/* step: ensure the perms to read only */
				fs.Chmod(os.FileMode(DEFAULT_FILE_PERMS))
			}
		}
	}
	return nil
}

func (r *StoreFS) HashString(content string) string {
	hasher := md5.New()
	io.WriteString(hasher, content)
	return string(hasher.Sum(nil))
}

func (r *StoreFS) Delete(path string) error {
	glog.V(VERBOSE_LEVEL).Infof("Delete() deleting the file: %s", path)
	if !r.Exists(path) {
		glog.Errorf("The file: %s does not exist", path)
		return FileDoesNotExistErr
	}
	if !r.IsFile(path) {
		glog.Errorf("The file: %s is not a file", path)
		return NotFileErr
	}
	/* attempt to delete the file */
	if err := os.Remove(path); err != nil {
		glog.Errorf("Failed to remove file: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *StoreFS) List(path string) ([]string, error) {
	list := make([]string, 0)
	/* step: check the path is a directory */
	if !r.Exists(path) {
		glog.Errorf("List() the path: %s does not exists", path)
		return nil, DoesNotExistErr
	}
	if !r.IsDirectory(path) {
		glog.Errorf("List() the path: %s is not a directory", path)
		return nil, DirectoryDoesNotExistErr
	}
	if files, err := ioutil.ReadDir(path); err != nil {
		glog.Errorf("Failed to get a listing of directory: %s, error: %s", path, err)
		return nil, err
	} else {
		for _, file := range files {
			list = append(list, file.Name())
		}
	}
	return list, nil
}

func (r *StoreFS) Exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (r *StoreFS) Stat(path string) (os.FileInfo, error) {
	if stat, err := os.Stat(path); err != nil {
		glog.Errorf("Failed to stat the path: %s, error: %s", path, err)
		return nil, err
	} else {
		return stat, nil
	}
}

func (r *StoreFS) IsDirectory(path string) bool {
	if found := r.Exists(path); !found {
		glog.Errorf("IsDirectory() path: %s does not exist", path)
		return false
	}
	if stat, err := r.Stat(path); err != nil {
		glog.Errorf("Failed to stat the file: %s, error: %s", path, err)
		return false
	} else {
		return stat.IsDir()
	}
}

func (r *StoreFS) IsFile(path string) bool {
	if found := r.Exists(path); !found {
		glog.Errorf("IsFile() the file: %s does not exist", path)
		return false
	}
	stat, err := r.Stat(path)
	if err != nil {
		glog.Errorf("Failed to stat the file: %s, error: %s", path, err)
		return false
	}
	return !stat.IsDir()
}

func (r *StoreFS) Mkdir(path string) error {
	parentDirectory := filepath.Dir(path)
	/* step: check the directory exists */
	if !r.Exists(parentDirectory) {
		glog.Errorf("Failed to create directory, parent directory: %s does not exists", path, parentDirectory)
		return errors.New("The parent directory does not exists")
	}
	if !r.IsDirectory(parentDirectory) {
		glog.Errorf("Failed to create directory: %s, parent: %s is not a directorty", path, parentDirectory)
		return errors.New("The parent is not a directory")
	}
	if err := os.Mkdir(path, os.FileMode(DEFAULT_DIRECTORY_PERMS)); err != nil {
		glog.Errorf("Failed to create the directory: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *StoreFS) Mkdirp(path string) error {
	if err := os.MkdirAll(path, os.FileMode(DEFAULT_DIRECTORY_PERMS)); err != nil {
		glog.Errorf("Failed to create the directory: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *StoreFS) Rmdir(path string) error {
	if !r.Exists(path) {
		glog.Errorf("Failed to delete directory: %s, the path does not exist", path)
		return DirectoryDoesNotExistErr
	}
	if !r.IsDirectory(path) {
		glog.Errorf("Failed to delete directory: %s, the path is not a directory", path)
		return DirectoryDoesNotExistErr
	}
	if err := os.RemoveAll(path); err != nil {
		glog.Errorf("Failed to remove the directory: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *StoreFS) Dirname(path string) string {
	return filepath.Dir(path)
}

const FILE_CHUNKS = 8192

func (r *StoreFS) Hash(path string) (string, error) {
	if !r.Exists(path) {
		glog.Errorf("Failed to hash file: %s, the path does not exist", path)
		return "", FileDoesNotExistErr
	}
	if !r.IsFile(path) {
		glog.Errorf("Failed to hash file: %s, the path is not a file", path)
		return "", FileDoesNotExistErr
	}
	if file, err := os.Open(path); err != nil {
		glog.Errorf("Failed to open the fileL: %s, error: %s", path, err)
		return "", err
	} else {
		defer file.Close()
		info, _ := file.Stat()
		filesize := info.Size()
		blocks := uint64(math.Ceil(float64(filesize) / float64(FILE_CHUNKS)))
		hash := md5.New()
		for i := uint64(0); i < blocks; i++ {
			blocksize := int(math.Min(FILE_CHUNKS, float64(filesize-int64(i*FILE_CHUNKS))))
			buf := make([]byte, blocksize)
			file.Read(buf)
			io.WriteString(hash, string(buf))
		}
		return string(hash.Sum(nil)[:]), nil
	}
}

func (r *StoreFS) Touch(path string) error {
	if !r.IsFile(path) {
		glog.Errorf("Failed to hash file: %s, the path is not a file", path)
		return FileDoesNotExistErr
	}
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		glog.Errorf("Failed to update stats on file: %s, error: %s", path, err)
		return err
	}
	return nil
}

func (r *StoreFS) ListDirectories(path string) ([]string, error) {
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
