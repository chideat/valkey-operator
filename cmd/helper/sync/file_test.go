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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// getTestLogger creates a logger for testing purposes
func getTestLogger() logr.Logger {
	return zap.New(zap.UseDevMode(true), zap.StacktraceLevel(zapcore.FatalLevel))
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
	tempDir, err := os.MkdirTemp("", "filewatcher-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

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
	handlerCalled := make(chan *FileStat, 10)

	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		handlerCalled <- fs
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filewatcher-run-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file to watch
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("initial content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFilePath)
	require.NoError(t, err)

	// Run the watcher in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := fw.Run(ctx)
		assert.NoError(t, err)
	}()

	// Wait for initial handler call (from watch method)
	select {
	case fs := <-handlerCalled:
		assert.Equal(t, testFilePath, fs.FilePath())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for initial handler call")
	}

	// Modify the file to trigger a write event
	time.Sleep(200 * time.Millisecond) // Small delay to ensure watcher is ready
	err = os.WriteFile(testFilePath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Wait for handler call after modification
	select {
	case fs := <-handlerCalled:
		assert.Equal(t, testFilePath, fs.FilePath())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for handler call after file modification")
	}

	// Remove the file to test removal event
	err = os.Remove(testFilePath)
	require.NoError(t, err)

	// Create a new file with the same name to test create event after removal
	time.Sleep(200 * time.Millisecond)
	err = os.WriteFile(testFilePath, []byte("new content"), 0644)
	require.NoError(t, err)

	// Wait for handler call after recreation
	select {
	case fs := <-handlerCalled:
		assert.Equal(t, testFilePath, fs.FilePath())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for handler call after file recreation")
	}
}

func TestFileWatcherRunWithMultipleFiles(t *testing.T) {
	// Create channels to signal when the handler is called
	handlerCalled := make(chan *FileStat, 10)

	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		fmt.Printf("File modified: %s\n", fs.FilePath())
		handlerCalled <- fs
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filewatcher-multiple-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

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

	// Run the watcher in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := fw.Run(ctx)
		assert.NoError(t, err)
	}()

	// Wait for initial handler calls (from watch method)
	filesWatched := make(map[string]bool)

	// We should get notifications for both files
	for range 4 {
		select {
		case fs := <-handlerCalled:
			filesWatched[fs.FilePath()] = true
		case <-time.After(3 * time.Second):
			continue
		}
	}

	assert.True(t, filesWatched[testFile1Path], "File 1 was not watched")
	assert.True(t, filesWatched[testFile2Path], "File 2 was not watched")

	// Test modifying both files
	time.Sleep(200 * time.Millisecond)
	err = os.WriteFile(testFile1Path, []byte("file 1 modified"), 0644)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	err = os.WriteFile(testFile2Path, []byte("file 2 modified"), 0644)
	require.NoError(t, err)

	// Clear the map for new checks
	filesWatched = make(map[string]bool)

	// We should get notifications for both files again
	for range 4 {
		select {
		case fs := <-handlerCalled:
			filesWatched[fs.FilePath()] = true
		case <-time.After(3 * time.Second):
			continue
		}
	}

	assert.True(t, filesWatched[testFile1Path], "File 1 modification was not detected")
	assert.True(t, filesWatched[testFile2Path], "File 2 modification was not detected")
}

func TestFileWatcherHandlerError(t *testing.T) {
	// Create a handler that returns an error
	logger := getTestLogger()
	handlerErrorCalled := make(chan struct{}, 1)

	handler := func(ctx context.Context, fs *FileStat) error {
		handlerErrorCalled <- struct{}{}
		return assert.AnError // Return a test error
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filewatcher-error-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file to watch
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFilePath)
	require.NoError(t, err)

	// Run the watcher in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := fw.Run(ctx)
		assert.NoError(t, err)
	}()

	// Wait for handler to be called and error
	select {
	case <-handlerErrorCalled:
		// Handler was called and returned an error as expected
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for handler error")
	}

	// The watcher should continue running even after a handler error
	time.Sleep(200 * time.Millisecond)
	err = os.WriteFile(testFilePath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Wait for handler to be called again
	select {
	case <-handlerErrorCalled:
		// Handler was called again despite previous error
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for handler call after error")
	}
}

func TestFileWatcherContextCancellation(t *testing.T) {
	logger := getTestLogger()
	handler := func(ctx context.Context, fs *FileStat) error {
		return nil
	}

	fw, err := NewFileWatcher(handler, logger)
	require.NoError(t, err)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filewatcher-context-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file to watch
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = fw.Add(testFilePath)
	require.NoError(t, err)

	// Run the watcher with a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Channel to check if Run has exited
	done := make(chan struct{})
	go func() {
		err := fw.Run(ctx)
		assert.NoError(t, err)
		close(done)
	}()

	// Give the watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Cancel the context to stop the watcher
	cancel()

	// Check if Run has exited
	select {
	case <-done:
		// Run has exited as expected
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for Run to exit after context cancellation")
	}
}
