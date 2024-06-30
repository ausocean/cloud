package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ausocean/utils/logging"
)

// Delay between changing a file and the file watcher picking up the change.
const watchTimeAllowance = 1 * time.Second

// TestWatchFile tests the watchFile function. It creates a temporary file,
// watches it, writes to it, and checks if the onWrite function was called.
func TestWatchFile(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("could not create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// We'll check this to see if the onWrite function was called.
	called := false

	err = watchFile(tmpFile.Name(), func() {
		called = true
	}, (*logging.TestLogger)(t))
	if err != nil {
		t.Fatalf("watchFile failed: %v", err)
	}

	if _, err := tmpFile.Write([]byte("hello world")); err != nil {
		t.Fatalf("could not write to temporary file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("could not close temporary file: %v", err)
	}

	// Allow some time for the file watcher to pick up the change.
	time.Sleep(watchTimeAllowance)

	if !called {
		t.Errorf("onWrite was not called after modifying the file")
	}
}

// TestWatchFileFileNotExistYet tests the watchFile function in the case
// that the file to be watched does not exist on the first call to watchFile.
// It creates a temporary directory, watches a file in that directory, creates
// and writes to the file, and checks if the onWrite function was called.
func TestWatchFileFileNotExistYet(t *testing.T) {
	// Create a temporary directory.
	tmpDir, err := ioutil.TempDir("", "example")
	if err != nil {
		t.Fatalf("could not create temporary directory: %v", err)
	}
	defer os.Remove(tmpDir) // clean up

	// File that does not exist yet but will be created in the temporary directory.
	fileName := filepath.Join(tmpDir, "testfile")

	called := false
	err = watchFile(fileName, func() {
		called = true
	}, (*logging.TestLogger)(t))
	if err != nil {
		t.Fatalf("watchFile failed: %v", err)
	}

	// Create and write to the file.
	err = ioutil.WriteFile(fileName, []byte("hello world"), 0666)
	if err != nil {
		t.Fatalf("could not write to file: %v", err)
	}

	// Allow some time for the file watcher to pick up the change.
	time.Sleep(watchTimeAllowance)

	if !called {
		t.Errorf("onWrite was not called after creating and modifying the file")
	}
}

// TestWatchFileMultipleChanges tests the watchFile function in the case
// that the file to be watched is modified multiple times. It creates a
// temporary file, watches it, writes to it twice, and checks if the onWrite
// function was called twice.
func TestWatchFileMultipleChanges(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("could not create temporary file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	// We'll count how many times onWrite was called.
	calledCount := 0

	err = watchFile(tmpfile.Name(), func() {
		calledCount++
	}, (*logging.TestLogger)(t))
	if err != nil {
		t.Fatalf("watchFile failed: %v", err)
	}

	// Write to the file twice.
	for i := 0; i < 2; i++ {
		if _, err := tmpfile.Write([]byte("hello world")); err != nil {
			t.Fatalf("could not write to temporary file: %v", err)
		}
		if err := tmpfile.Sync(); err != nil {
			t.Fatalf("could not sync temporary file: %v", err)
		}

		// Allow some time for the file watcher to pick up the change.
		time.Sleep(watchTimeAllowance)
	}

	if calledCount != 2 {
		t.Errorf("onWrite was not called the expected number of times after modifying the file")
	}
}
