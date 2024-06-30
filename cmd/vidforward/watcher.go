/*
DESCRIPTION

	watcher.go provides a tool for watching a file for modifications and
	performing an action when the file is modified.

AUTHORS

	Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package main

import (
	"fmt"
	"path"

	"github.com/ausocean/utils/logging"
	"github.com/fsnotify/fsnotify"
)

// watchFile watches a file for modifications and calls onWrite when the file
// is modified. Technically, the directory is watched instead of the file.
// This is because watching the file itself will cause problems if changes
// are done atomically.
// See fsnotify documentation:
// https://godocs.io/github.com/fsnotify/fsnotify#hdr-Watching_files
func watchFile(file string, onWrite func(), l logging.Logger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("could not create watcher: %w", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					l.Warning("watcher events chan closed, terminating")
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write && event.Name == file {
					l.Info("file modification event", "file", file)
					onWrite()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					l.Warning("watcher error chan closed, terminating")
					return
				}
				l.Error("file watcher error", "error", err)
			}
		}
	}()

	// Watch the directory over the file.
	err = watcher.Add(path.Dir(file))
	if err != nil {
		return fmt.Errorf("could not add file %s to watcher: %w", file, err)
	}
	return nil
}
