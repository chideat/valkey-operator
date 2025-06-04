/*
Copyright 2024 chideat.

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

package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
)

type FileStat struct {
	isWatching bool

	filepath            string
	registerTimestamp   int64
	lastUpdateTimestamp int64

	lock sync.Mutex
}

func (fs *FileStat) FilePath() string {
	return fs.filepath
}

func (fs *FileStat) LastUpdateTimestamp() int64 {
	return fs.lastUpdateTimestamp
}

type FileEventHandler func(ctx context.Context, fs *FileStat) error

type FileWatcher struct {
	watchingFiles sync.Map
	logger        logr.Logger

	handler FileEventHandler
}

func NewFileWatcher(handler FileEventHandler, logger logr.Logger) (*FileWatcher, error) {
	w := FileWatcher{
		handler: handler,
		logger:  logger.WithName("FileWatcher"),
	}
	return &w, nil
}

func (w *FileWatcher) Add(filepath string) error {
	if info, err := os.Stat(filepath); err != nil {
		if os.IsNotExist(err) {
			w.logger.V(2).Info("file not exists", "file", filepath)
		} else {
			w.logger.Error(err, "check file failed", "file", filepath)
			return err
		}
	} else if info.IsDir() {
		return errors.New("only file supported")
	}

	if _, exists := w.watchingFiles.LoadOrStore(filepath, &FileStat{
		isWatching:        false,
		filepath:          filepath,
		registerTimestamp: time.Now().Unix(),
	}); exists {
		w.logger.V(2).Info("file has already registered", "file", filepath)
	}
	return nil
}

func (w *FileWatcher) watch(ctx context.Context, watcher *fsnotify.Watcher, fs *FileStat) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	if fs.isWatching {
		return nil
	}

	if fileStat, err := os.Stat(fs.filepath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		w.logger.Error(err, "check file state failed", "file", fs.filepath)
		return err
	} else if fileStat.IsDir() {
		return fmt.Errorf("dir not supported")
	}
	if err := watcher.Add(fs.filepath); err != nil {
		return err
	}

	// emit create event
	fs.lastUpdateTimestamp = time.Now().Unix()
	if w.handler != nil {
		if err := w.handler(ctx, fs); err != nil {
			w.logger.Error(err, "handle file failed")
		}
	}
	fs.isWatching = true

	return nil
}

func (w *FileWatcher) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	checker := func(ctx context.Context) {
		w.watchingFiles.Range(func(key, value any) bool {
			fs, _ := value.(*FileStat)
			if err := w.watch(ctx, watcher, fs); err != nil {
				w.logger.Error(err, "watch file state failed", "file", fs.filepath)
			}
			return true
		})
	}

	checker(ctx)

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			checker(ctx)
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			func() {
				val, _ := w.watchingFiles.Load(event.Name)

				fs, _ := val.(*FileStat)
				fs.lock.Lock()
				defer fs.lock.Unlock()

				switch event.Op {
				case fsnotify.Create, fsnotify.Write, fsnotify.Rename:
					fs.lastUpdateTimestamp = time.Now().Unix()

					if w.handler != nil {
						if err := w.handler(ctx, fs); err != nil {
							w.logger.Error(err, "handle file failed")
						}
					}
				case fsnotify.Remove:
					fs.isWatching = false
				}
			}()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}
