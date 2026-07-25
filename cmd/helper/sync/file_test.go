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
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	// eventTimeout bounds how long a test waits for a watcher callback.
	//
	// It is a backstop against a hang, not a statement about how quickly the
	// watcher ought to react: a passing test returns the moment the event
	// arrives, so a generous bound costs nothing and only decides how long a
	// genuinely stuck watcher takes to be reported.
	eventTimeout = 30 * time.Second

	// settleDelay spaces out consecutive writes to the same file.
	//
	// The filesystem coalesces writes that land close together, so without a
	// gap the second write can be folded into the first notification and the
	// test would wait for an event that is never going to arrive. This is
	// about filesystem behaviour rather than about giving the watcher time to
	// catch up, which is why it stays a sleep.
	settleDelay = 200 * time.Millisecond

	// eventBuffer sizes the channels the handlers report on. It only has to be
	// deep enough that a test reading in a loop is never the reason the
	// watcher stalls; see sendEvent for what happens once it is full.
	eventBuffer = 64
)

// getTestLogger creates a logger for testing purposes
func getTestLogger() logr.Logger {
	return zap.New(zap.UseDevMode(true), zap.StacktraceLevel(zapcore.FatalLevel))
}

// startWatcher runs fw for the duration of the test.
//
// Cleanup cancels the context and then waits for Run to return. The waiting is
// the point: Run owns an fsnotify watcher, and that watcher and its file
// descriptors stay live until Run returns. These tests used to start Run and
// walk away, so every test left a watcher behind; under `go test -count`,
// enough of them accumulated that later tests missed filesystem events
// entirely and failed on a watcher that was working correctly.
func startWatcher(t *testing.T, fw *FileWatcher) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		assert.NoError(t, fw.Run(ctx))
	}()

	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(eventTimeout):
			t.Error("watcher did not stop within " + eventTimeout.String() + " of cancelling its context")
		}
	})
}

// sendEvent reports a callback to a test without ever blocking the watcher.
//
// The handler runs on the watcher's own goroutine while holding the file's
// lock, so a blocking send stalls the watcher for as long as nobody reads --
// and once a test stops reading, stalls it forever, which is how these tests
// leaked their watchers. Discarding an event when the buffer is full is safe
// here because every assertion is about a path being reported, never about how
// many times it was reported.
func sendEvent[T any](events chan<- T, value T) {
	select {
	case events <- value:
	default:
	}
}

// awaitEvent returns the next value sent on events, failing the test if none
// arrives within eventTimeout. what names the step being waited on so that a
// failure says which one timed out.
func awaitEvent[T any](t *testing.T, events <-chan T, what string) T {
	t.Helper()

	select {
	case value := <-events:
		return value
	case <-time.After(eventTimeout):
		t.Fatalf("timed out after %s waiting for %s", eventTimeout, what)
		var zero T
		return zero
	}
}

// awaitPaths consumes callbacks until every wanted path has been reported.
//
// The watcher promises nothing about how many callbacks a batch of writes
// produces -- it may coalesce them, and it may report a path more than once --
// so waiting on the set of paths the test cares about is both what the test
// means and the only formulation that cannot fail spuriously. Waiting for a
// fixed number of events, as this did before, gives up as soon as that count
// is reached even when a path has still not been seen.
func awaitPaths(t *testing.T, events <-chan *FileStat, what string, want ...string) {
	t.Helper()

	pending := make(map[string]struct{}, len(want))
	for _, path := range want {
		pending[path] = struct{}{}
	}

	deadline := time.After(eventTimeout)
	for len(pending) > 0 {
		select {
		case stat := <-events:
			delete(pending, stat.FilePath())
		case <-deadline:
			t.Fatalf("timed out after %s waiting for %s; no callback for %v",
				eventTimeout, what, slices.Sorted(maps.Keys(pending)))
		}
	}
}

func TestClassifyEvent(t *testing.T) {
	cases := []struct {
		name string
		op   fsnotify.Op
		want fileEventKind
	}{
		{name: "write", op: fsnotify.Write, want: fileUpdated},
		{name: "create", op: fsnotify.Create, want: fileUpdated},
		{name: "rename", op: fsnotify.Rename, want: fileUpdated},
		{name: "remove", op: fsnotify.Remove, want: fileRemoved},
		{name: "chmod alone says nothing about the contents", op: fsnotify.Chmod, want: fileIgnored},

		// The kernel reports several operations in one event when they land
		// together. Rewriting a file in place is routinely delivered as
		// WRITE|CHMOD, and treating Op as a single value dropped it outright,
		// so the watcher missed the change completely.
		{name: "write coalesced with chmod", op: fsnotify.Write | fsnotify.Chmod, want: fileUpdated},
		{name: "create coalesced with chmod", op: fsnotify.Create | fsnotify.Chmod, want: fileUpdated},
		{name: "create coalesced with write", op: fsnotify.Create | fsnotify.Write, want: fileUpdated},
		{name: "remove coalesced with chmod", op: fsnotify.Remove | fsnotify.Chmod, want: fileRemoved},

		{name: "no bits set", op: 0, want: fileIgnored},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyEvent(tc.op); got != tc.want {
				t.Errorf("classifyEvent(%s) = %v, want %v", tc.op, got, tc.want)
			}
		})
	}
}

func TestFileStatMethods(t *testing.T) {
	fs := &FileStat{
		filepath:            "/test/path",
		lastUpdateTimestamp: 123456789,
	}

	assert.Equal(t, "/test/path", fs.FilePath())
	assert.Equal(t, int64(123456789), fs.LastUpdateTimestamp())
}

func TestNewFileWatcher(t *testing.T) {
	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)
	assert.NotNil(t, fw)
}

func TestFileWatcherAdd(t *testing.T) {
	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Test adding a valid file
	validFilePath := filepath.Join(tempDir, "valid-file.txt")
	err = os.WriteFile(validFilePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = fw.Add(validFilePath)
	assert.NoError(t, err)

	// Test adding the same file again (should not error)
	err = fw.Add(validFilePath)
	assert.NoError(t, err)

	// Test adding a non-existent file (should not error, but log a message)
	nonExistentPath := filepath.Join(tempDir, "non-existent.txt")
	err = fw.Add(nonExistentPath)
	assert.NoError(t, err)

	// Test adding a directory (should error)
	err = fw.Add(tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only file supported")
}

func TestFileWatcherRun(t *testing.T) {
	// Create a channel to signal when the handler is called
	handlerCalled := make(chan *FileStat, eventBuffer)

	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		sendEvent(handlerCalled, fs)
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a file to watch
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("initial content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFilePath)
	require.NoError(t, err)

	startWatcher(t, fw)

	// Wait for initial handler call (from watch method)
	stat := awaitEvent(t, handlerCalled, "the initial handler call")
	assert.Equal(t, testFilePath, stat.FilePath())

	// Modify the file to trigger a write event
	time.Sleep(settleDelay)
	err = os.WriteFile(testFilePath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Wait for handler call after modification
	stat = awaitEvent(t, handlerCalled, "the handler call after file modification")
	assert.Equal(t, testFilePath, stat.FilePath())

	// Remove the file to test removal event
	err = os.Remove(testFilePath)
	require.NoError(t, err)

	// Create a new file with the same name to test create event after removal
	time.Sleep(settleDelay)
	err = os.WriteFile(testFilePath, []byte("new content"), 0644)
	require.NoError(t, err)

	// Wait for handler call after recreation
	stat = awaitEvent(t, handlerCalled, "the handler call after file recreation")
	assert.Equal(t, testFilePath, stat.FilePath())
}

func TestFileWatcherRunWithMultipleFiles(t *testing.T) {
	// Create channels to signal when the handler is called
	handlerCalled := make(chan *FileStat, eventBuffer)

	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		sendEvent(handlerCalled, fs)
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create multiple files to watch
	testFile1Path := filepath.Join(tempDir, "test-file1.txt")
	err = os.WriteFile(testFile1Path, []byte("file 1 content"), 0644)
	require.NoError(t, err)

	testFile2Path := filepath.Join(tempDir, "test-file2.txt")
	err = os.WriteFile(testFile2Path, []byte("file 2 content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFile1Path)
	require.NoError(t, err)

	err = fw.Add(testFile2Path)
	require.NoError(t, err)

	startWatcher(t, fw)

	// Wait for initial handler calls (from watch method)
	awaitPaths(t, handlerCalled, "both files to be watched", testFile1Path, testFile2Path)

	// Test modifying both files
	time.Sleep(settleDelay)
	err = os.WriteFile(testFile1Path, []byte("file 1 modified"), 0644)
	require.NoError(t, err)

	time.Sleep(settleDelay)
	err = os.WriteFile(testFile2Path, []byte("file 2 modified"), 0644)
	require.NoError(t, err)

	// We should get notifications for both files again
	awaitPaths(t, handlerCalled, "both modifications to be detected", testFile1Path, testFile2Path)
}

func TestFileWatcherHandlerError(t *testing.T) {
	// Create a handler that returns an error
	logger := getTestLogger()
	handlerErrorCalled := make(chan struct{}, eventBuffer)

	handler := func(ctx context.Context, fs *FileStat) error {
		sendEvent(handlerErrorCalled, struct{}{})
		return assert.AnError // Return a test error
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a file to watch
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFilePath)
	require.NoError(t, err)

	startWatcher(t, fw)

	// Wait for handler to be called and error
	awaitEvent(t, handlerErrorCalled, "the handler to be called and return an error")

	// The watcher should continue running even after a handler error
	time.Sleep(settleDelay)
	err = os.WriteFile(testFilePath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Wait for handler to be called again
	awaitEvent(t, handlerErrorCalled, "the handler call after a previous error")
}

func TestFileWatcherContextCancellation(t *testing.T) {
	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a file to watch
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFilePath)
	require.NoError(t, err)

	// Run the watcher with a context that we'll cancel. This test drives the
	// lifecycle itself rather than using startWatcher, because cancellation is
	// what it is asserting on.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to check if Run has exited
	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, fw.Run(ctx))
	}()

	// Give the watcher time to start, so that cancellation is exercised
	// against a running watcher rather than against a context that was
	// already done before Run first looked at it.
	time.Sleep(settleDelay)

	// Cancel the context to stop the watcher
	cancel()

	// Check if Run has exited
	awaitEvent(t, done, "Run to exit after context cancellation")
}
