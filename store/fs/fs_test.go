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
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	fsStore FileStore
	tmpDir  string
)

const (
	TEST_DIRECTORY = "/testdir"
	TEST_FILE      = "/testfile"
)

func testPath(path string) string {
	return fmt.Sprintf("%s%s", tmpDir, path)
}

func testDirectory() string {
	return testPath(TEST_DIRECTORY)
}

func testFile() string {
	return testPath(TEST_FILE)
}

func testFiles(t *testing.T) {
	if err := os.Mkdir(testPath(TEST_DIRECTORY), os.FileMode(DEFAULT_DIRECTORY_PERMS)); err != nil {
		t.Fatalf("Failed to create a directory, error: %s", err)
	}
	if file, err := os.Create(testPath(TEST_FILE)); err != nil {
		t.Fatalf("Failed to create a directory, error: %s", err)
	} else {
    	file.Close()
	}
}

func checkExists(path string, t *testing.T) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("Unable to test the existance of path: %s, error: %s", path, err)
	}
	return true
}

func checkContent(path, value string, t *testing.T) bool {
	if content, err := ioutil.ReadFile(path); err != nil {
		t.Fatalf("Unable to read the content from file: %s, error: %s", path, err)
	} else {
		if string(content) == value {
			return true
		}
	}
	return false
}

func TestSetup(t *testing.T) {
	var err error
	tmpDir, err = ioutil.TempDir("/tmp", "configfs")
	if err != nil {
		t.Fatalf("Failed to create the temporary directory, error: %s", err)
	}
	testFiles(t)
	fsStore = NewStoreFS()
}

func TestCreate(t *testing.T) {
	path := testPath("/create_test")
	err := fsStore.Create(path, "test")
	assert.Nil(t, err, "unable to create the file, error: %s", err)
	assert.True(t, checkExists(path, t), "the file does not exists")
	assert.False(t, checkContent(path, "test2", t), "the content check should have returned false")
	assert.True(t, checkContent(path, "test", t), "the content of the file is the not same")
}

func TestUpdate(t *testing.T) {
	path := testPath("/create_test")
	err := fsStore.Update(path, "test_updated")
	assert.Nil(t, err, "unable to update the file, error: %s", err)
	assert.True(t, checkExists(path, t), "the file: %s does not exists", path)
	assert.True(t, checkContent(path, "test_updated", t), "the content of the file is the not same")
}

func TestHash(t *testing.T) {
	path := testPath("/create_test")
	hash, err := fsStore.Hash(path)
	assert.Nil(t, err, "unable to compute the hash of the file, error: %s", err)
	assert.Equal(t, "~\xef\xae\xf7q\xeb^\x00t\xde\xf2r\x8e\xa0\x0f\xd5", hash, "the hash computed is the not same")
}

func TestDelete(t *testing.T) {
	path := testPath("/create_test")
	err := fsStore.Delete(path)
	assert.Nil(t, err, "unable to delete the file: %s, error: %s", path, err)
	assert.False(t, checkExists(path, t), "the file should no longer exist")
}

func TestList(t *testing.T) {
	path1 := testPath("/create_test")
	path2 := testPath("/create_test1")
	err := fsStore.Create(path1, "test")
	assert.Nil(t, err, "unable to create the file: %s, error: %s", path1, err)
	assert.True(t, checkExists(path1, t), "the file: %s does not exists", path1)
	err = fsStore.Create(path2, "test")
	assert.Nil(t, err, "unable to create the file: %s, error: %s", path2, err)
	assert.True(t, checkExists(path2, t), "the file: %s does not exists", path2)
	listings, err := fsStore.List(testPath("/"))
	assert.Nil(t, err, "unable to get listing of directory, error: %s", err)
	assert.NotNil(t, listings, "the directory listing is nil")
	assert.NotEmpty(t, listings, "the directory listing should not be empty")
	assert.Equal(t, "create_test", listings[0])
	assert.Equal(t, "create_test1", listings[1])
}

func TestExists(t *testing.T) {
	path := testPath("/create_test")
	found := fsStore.Exists(path)
	assert.True(t, found, "the file: %s does not exists", path)
	found = fsStore.Exists("/should_not_be_there")
	assert.False(t, found, "the file not exist, this should be false")
}

func TestIsDirectory(t *testing.T) {
	value := fsStore.IsDirectory(testDirectory())
	assert.True(t, value, "the should have been true, directory exists")
	value = fsStore.IsDirectory(testPath("/nothing"))
	assert.False(t, value, "the should have been false, directory does not exist")
	value = fsStore.IsDirectory(testFile())
	assert.False(t, value, "the should have been false, this is a file")
}

func TestIsFile(t *testing.T) {
	value := fsStore.IsFile(testFile())
	assert.True(t, value, "the should have been true, file exists")
	value = fsStore.IsFile(testPath("/nothing"))
	assert.False(t, value, "the should have been false, file does not exist")
	value = fsStore.IsFile(testDirectory())
	assert.False(t, value, "the should have been false, path is a directory")
}

func TestMkdir(t *testing.T) {
	directory := "/testing_directory"
	err := fsStore.Mkdir(testPath(directory))
	assert.Nil(t, err, "unable to create a directory, error: %s", err)
	assert.True(t, checkExists(testPath(directory), t), "the directory was not created")
}

func TestMkdirp(t *testing.T) {
	directory := "/testing_directory/t1/t2"
	err := fsStore.Mkdirp(testPath(directory))
	assert.Nil(t, err, "unable to create a directories, error: %s", err)
	assert.True(t, checkExists(testPath(directory), t), "the directories was not created")
}

func TestRmdir(t *testing.T) {
	directory := "/testing_directory"
	err := fsStore.Rmdir(testPath(directory))
	assert.Nil(t, err, "unable to remove all the directories, error: %s", err)
	assert.False(t, checkExists(testPath(directory), t), "the directories were not deleted")
}

func TestTouch(t *testing.T) {
	file := "/testing_touch"
	err := fsStore.Touch(testPath(file))
	assert.NotNil(t, err, "this should have thrown an error")
	assert.Nil(t, fsStore.Touch(testFile()), "unable to touch the file, error")
}

func TestTearDown(t *testing.T) {
	os.RemoveAll(tmpDir)
}
