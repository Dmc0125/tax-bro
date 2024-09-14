package walletsync

import (
	"sync"
	"testing"
)

func TestWalk(t *testing.T) {
	q := newMsgQueue()
	items := []int{1, 2, 3, 4, 5, 6}
	for _, i := range items {
		q.append(i)
	}

	t.Run("No deletion", func(t *testing.T) {
		count := 0
		q.walk(func(item interface{}) bool {
			count++
			return false
		}, func() bool { return false })
		if count != 6 {
			t.Errorf("Expected to walk 6 items, walked %d", count)
		}
	})

	t.Run("Delete even numbers", func(t *testing.T) {
		q.walk(func(item interface{}) bool {
			return item.(int)%2 == 0
		}, func() bool { return false })
		if len(q.items) != 3 {
			t.Errorf("Expected 3 items after deletion, got %d", len(q.items))
		}
		for _, item := range q.items {
			if item.(int)%2 == 0 {
				t.Errorf("Found even number %d after deletion", item)
			}
		}
	})

	t.Run("Exit early", func(t *testing.T) {
		count := 0
		q.walk(func(item interface{}) bool {
			count++
			return false
		}, func() bool { return count >= 2 })
		if count != 2 {
			t.Errorf("Expected to exit after 2 items, walked %d", count)
		}
	})
}

func TestConcurrency(t *testing.T) {
	q := newMsgQueue()
	numGoroutines := 100
	itemsPerGoroutine := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				q.append(j)
			}
		}()
	}

	wg.Wait()

	if len(q.items) != numGoroutines*itemsPerGoroutine {
		t.Errorf("Expected %d items, got %d", numGoroutines*itemsPerGoroutine, len(q.items))
	}
}
