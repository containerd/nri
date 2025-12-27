/*
   Copyright The containerd Authors.

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

package adaptation

import (
	"context"
	"sync"
)

// UnlockFunc is a function returned by Lock methods to release the acquired lock.
type UnlockFunc func()

// locker defines the internal interface for locking strategies.
type locker interface {
	// Lock acquires the global lock.
	Lock(ctx context.Context) UnlockFunc
	// LockPod acquires the lock for a specific pod.
	LockPod(ctx context.Context, podUID string) UnlockFunc
	// CleanupPod performs cleanup for a pod lock.
	CleanupPod(ctx context.Context, podUID string)
}

var _ locker = &globalLocker{}

// globalLocker implements locker using a single global mutex.
type globalLocker struct {
	mu sync.Mutex
}

// newGlobalLocker creates the default global locker.
func newGlobalLocker() locker {
	return &globalLocker{}
}

func (m *globalLocker) Lock(context.Context) UnlockFunc {
	m.mu.Lock()
	return m.mu.Unlock
}

func (m *globalLocker) LockPod(context.Context, string) UnlockFunc {
	// Ignores podUID, uses the single global lock
	m.mu.Lock()
	return m.mu.Unlock
}

func (m *globalLocker) CleanupPod(context.Context, string) {
	// No-op
}

var _ locker = &podLocker{}

// podLocker implements locker using a separate mutex for each Pod UID
// and using its own main mutex 'mu' as the global lock.
type podLocker struct {
	mu    sync.Mutex             // Protects access to the locks map AND acts as the global lock.
	locks map[string]*sync.Mutex // Map from Pod UID to its mutex
}

// newPodLocker creates the per-pod locker.
func newPodLocker() locker {
	return &podLocker{
		locks: make(map[string]*sync.Mutex),
	}
}

// Lock acquires the main mutex, acting as the global lock.
func (m *podLocker) Lock(context.Context) UnlockFunc {
	m.mu.Lock()
	return m.mu.Unlock
}

// LockPod acquires the lock for a specific Pod UID.
func (m *podLocker) LockPod(_ context.Context, podUID string) UnlockFunc {
	m.mu.Lock()
	podMu, ok := m.locks[podUID]
	if !ok {
		podMu = &sync.Mutex{}
		m.locks[podUID] = podMu
	}
	m.mu.Unlock()

	podMu.Lock()
	return podMu.Unlock
}

// CleanupPod removes the lock for a specific Pod UID from the map.
func (m *podLocker) CleanupPod(_ context.Context, podUID string) {
	m.mu.Lock()
	delete(m.locks, podUID)
	m.mu.Unlock()
}
