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
	"fmt"
	"sync"
	"testing"
	"time"
)

const (
	testPod1 = "pod-uid-1"
	testPod2 = "pod-uid-2"
	numOps   = 1000
	numGR    = 10
)

var testCtx = context.Background()

func TestGlobalLocker_Concurrency(t *testing.T) {
	locker := newGlobalLocker()
	var counter int
	var wg sync.WaitGroup

	wg.Add(numGR)
	for i := 0; i < numGR; i++ {
		// Use different pod UIDs, but they should all contend for the same global lock
		podUID := fmt.Sprintf("pod-uid-%d", i)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				unlock := locker.LockPod(testCtx, id)
				counter++
				unlock()
			}
		}(podUID)
	}

	wg.Wait()

	expected := numGR * numOps
	if counter != expected {
		t.Errorf("Concurrency test failed: expected counter %d, got %d", expected, counter)
	}
}

func TestPodLocker_ConcurrencySamePod(t *testing.T) {
	locker := newPodLocker()
	var counter int
	var wg sync.WaitGroup

	wg.Add(numGR)
	for i := 0; i < numGR; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				unlock := locker.LockPod(testCtx, testPod1)
				counter++
				unlock()
			}
		}()
	}

	wg.Wait()

	expected := numGR * numOps
	if counter != expected {
		t.Errorf("Concurrency test (same pod) failed: expected counter %d, got %d", expected, counter)
	}
}

func TestPodLocker_ConcurrencyDifferentPods(t *testing.T) {
	// This test aims to show that different pods *can* run concurrently.
	// It's harder to definitively prove concurrency without timing, which is flaky.
	// Instead, we run operations for different pods concurrently and check for races.
	// If the per-pod lock isolates correctly, the race detector should not complain
	// about the separate counters.

	locker := newPodLocker()
	counters := make([]int, numGR)
	var wg sync.WaitGroup

	wg.Add(numGR)
	for i := 0; i < numGR; i++ {
		podUID := fmt.Sprintf("pod-uid-%d", i)
		counterIndex := i
		go func(id string, idx int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				unlock := locker.LockPod(testCtx, id)
				counters[idx]++
				time.Sleep(time.Microsecond)
				unlock()
			}
		}(podUID, counterIndex)
	}

	wg.Wait()

	for i, count := range counters {
		if count != numOps {
			t.Errorf("Concurrency test (different pods) failed for pod %d: expected counter %d, got %d", i, numOps, count)
		}
	}
}

func TestPodLocker_Cleanup(t *testing.T) {
	locker := newPodLocker().(*podLocker)

	unlock1 := locker.LockPod(testCtx, testPod1)
	unlock1()

	locker.mu.Lock()
	_, existsBefore := locker.locks[testPod1]
	locker.mu.Unlock()
	if !existsBefore {
		t.Fatalf("Lock for %s was not created", testPod1)
	}

	locker.CleanupPod(testCtx, testPod1)

	locker.mu.Lock()
	_, existsAfter := locker.locks[testPod1]
	locker.mu.Unlock()
	if existsAfter {
		t.Fatalf("Lock for %s was not removed by Cleanup", testPod1)
	}

	unlock2 := locker.LockPod(testCtx, testPod2)
	locker.CleanupPod(testCtx, testPod1)
	locker.mu.Lock()
	_, existsPod2 := locker.locks[testPod2]
	locker.mu.Unlock()
	if !existsPod2 {
		t.Fatalf("Cleanup for %s incorrectly removed lock for %s", testPod1, testPod2)
	}
	unlock2()

	unlockEmpty := locker.LockPod(testCtx, "")
	unlockEmpty()
	locker.CleanupPod(testCtx, "")
	locker.mu.Lock()
	_, existsEmpty := locker.locks["<no-pod-uid>"]
	locker.mu.Unlock()
	if existsEmpty {
		t.Fatalf("Lock for empty pod UID was not removed by Cleanup")
	}
}

func TestPodLocker_CleanupNoPanicOnNonExistent(t *testing.T) {
	locker := newPodLocker()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unlock on non-existent lock panicked unexpectedly: %v", r)
		}
	}()

	locker.CleanupPod(testCtx, testPod1)
	t.Log("Unlock on non-existent lock did not panic, as expected.")
}
