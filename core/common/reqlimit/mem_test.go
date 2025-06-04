package reqlimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/reqlimit"
)

func TestNewInMemoryRateLimiter(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()
	if rl == nil {
		t.Fatal("NewInMemoryRateLimiter should return a non-nil instance")
	}
}

func TestPushRequestBasic(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	normalCount, overCount, secondCount := rl.PushRequest(10, 60*time.Second, 1, "group1", "model1")

	if normalCount != 1 {
		t.Errorf("Expected normalCount to be 1, got %d", normalCount)
	}
	if overCount != 0 {
		t.Errorf("Expected overCount to be 0, got %d", overCount)
	}
	if secondCount != 1 {
		t.Errorf("Expected secondCount to be 1, got %d", secondCount)
	}
}

func TestPushRequestRateLimit(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	maxReq := int64(2)
	duration := 60 * time.Second

	for i := range 4 {
		normalCount, overCount, _ := rl.PushRequest(maxReq, duration, 1, "group1", "model1")

		switch {
		case i < 2:
			if normalCount != int64(i+1) {
				t.Errorf("Request %d: expected normalCount %d, got %d", i+1, i+1, normalCount)
			}
			if overCount != 0 {
				t.Errorf("Request %d: expected overCount 0, got %d", i+1, overCount)
			}
		case i == 2:
			if normalCount != 3 {
				t.Errorf("Request %d: expected normalCount 3, got %d", i+1, normalCount)
			}
			if overCount != 0 {
				t.Errorf("Request %d: expected overCount 0, got %d", i+1, overCount)
			}
		case i == 3:
			if normalCount != 3 {
				t.Errorf("Request %d: expected normalCount 3, got %d", i+1, normalCount)
			}
			if overCount != 1 {
				t.Errorf("Request %d: expected overCount 1, got %d", i+1, overCount)
			}
		}
	}
}

func TestPushRequestUnlimited(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	for i := range 5 {
		normalCount, overCount, _ := rl.PushRequest(0, 60*time.Second, 1, "group1", "model1")

		if normalCount != int64(i+1) {
			t.Errorf("Request %d: expected normalCount %d, got %d", i+1, i+1, normalCount)
		}
		if overCount != 0 {
			t.Errorf("Request %d: expected overCount 0, got %d", i+1, overCount)
		}
	}
}

func TestGetRequest(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	rl.PushRequest(10, 60*time.Second, 1, "group1", "model1")
	rl.PushRequest(10, 60*time.Second, 1, "group1", "model2")
	rl.PushRequest(10, 60*time.Second, 1, "group2", "model1")

	totalCount, secondCount := rl.GetRequest(60*time.Second, "group1", "model1")
	if totalCount != 1 {
		t.Errorf("Expected totalCount 1, got %d", totalCount)
	}
	if secondCount != 1 {
		t.Errorf("Expected secondCount 1, got %d", secondCount)
	}

	totalCount, _ = rl.GetRequest(60*time.Second, "*", "*")
	if totalCount != 3 {
		t.Errorf("Expected totalCount 3 for wildcard query, got %d", totalCount)
	}

	totalCount, _ = rl.GetRequest(60*time.Second, "group1", "*")
	if totalCount != 2 {
		t.Errorf("Expected totalCount 2 for group1 wildcard, got %d", totalCount)
	}
}

func TestMultipleGroupsAndModels(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	groups := []string{"group1", "group2", "group3"}
	models := []string{"model1", "model2"}

	for _, group := range groups {
		for _, model := range models {
			rl.PushRequest(10, 60*time.Second, 1, group, model)
		}
	}

	totalCount, _ := rl.GetRequest(60*time.Second, "*", "*")
	expected := len(groups) * len(models)
	if totalCount != int64(expected) {
		t.Errorf("Expected totalCount %d, got %d", expected, totalCount)
	}
}

func TestTimeWindowCleanup(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	rl.PushRequest(10, 2*time.Second, 1, "group1", "model1")

	totalCount, _ := rl.GetRequest(2*time.Second, "group1", "model1")
	if totalCount != 1 {
		t.Errorf("Expected totalCount 1, got %d", totalCount)
	}

	time.Sleep(3 * time.Second)

	totalCount, _ = rl.GetRequest(2*time.Second, "group1", "model1")
	if totalCount != 0 {
		t.Errorf("Expected totalCount 0 after cleanup, got %d", totalCount)
	}
}

func TestConcurrentAccess(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	const numGoroutines = 100
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(_ int) {
			defer wg.Done()
			for range requestsPerGoroutine {
				rl.PushRequest(0, 60*time.Second, 1, "group1", "model1")
			}
		}(i)
	}

	wg.Wait()

	totalCount, _ := rl.GetRequest(60*time.Second, "group1", "model1")
	expected := int64(numGoroutines * requestsPerGoroutine)
	if totalCount != expected {
		t.Errorf("Expected totalCount %d, got %d", expected, totalCount)
	}
}

func TestConcurrentDifferentKeys(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			group := fmt.Sprintf("group%d", id%5)
			model := fmt.Sprintf("model%d", id%3)
			rl.PushRequest(10, 60*time.Second, 1, group, model)
		}(i)
	}

	wg.Wait()

	// 验证总数
	totalCount, _ := rl.GetRequest(60*time.Second, "*", "*")
	if totalCount != int64(numGoroutines) {
		t.Errorf("Expected totalCount %d, got %d", numGoroutines, totalCount)
	}
}

func TestRateLimitWithOverflow(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	maxReq := 5
	duration := 60 * time.Second

	for i := range 10 {
		normalCount, overCount, _ := rl.PushRequest(int64(maxReq), duration, 1, "group1", "model1")

		if i < maxReq {
			if normalCount != int64(i+1) || overCount != 0 {
				t.Errorf("Request %d: expected normal=%d, over=0, got normal=%d, over=%d",
					i+1, i+1, normalCount, overCount)
			}
		} else {
			expectedOver := int64(i - maxReq)
			if normalCount != int64(maxReq+1) || overCount != expectedOver {
				t.Errorf("Request %d: expected normal=5, over=%d, got normal=%d, over=%d",
					i+1, expectedOver, normalCount, overCount)
			}
		}
	}
}

func TestEmptyQueries(t *testing.T) {
	rl := reqlimit.NewInMemoryRecord()

	totalCount, secondCount := rl.GetRequest(60*time.Second, "*", "*")
	if totalCount != 0 || secondCount != 0 {
		t.Errorf("Expected empty results, got total=%d, second=%d", totalCount, secondCount)
	}

	totalCount, secondCount = rl.GetRequest(60*time.Second, "nonexistent", "model")
	if totalCount != 0 || secondCount != 0 {
		t.Errorf(
			"Expected empty results for nonexistent key, got total=%d, second=%d",
			totalCount,
			secondCount,
		)
	}
}

func BenchmarkPushRequest(b *testing.B) {
	rl := reqlimit.NewInMemoryRecord()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			group := fmt.Sprintf("group%d", i%10)
			model := fmt.Sprintf("model%d", i%5)
			rl.PushRequest(100, 60*time.Second, 1, group, model)
			i++
		}
	})
}

func BenchmarkGetRequest(b *testing.B) {
	rl := reqlimit.NewInMemoryRecord()

	for i := range 100 {
		group := fmt.Sprintf("group%d", i%10)
		model := fmt.Sprintf("model%d", i%5)
		rl.PushRequest(100, 60*time.Second, 1, group, model)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			group := fmt.Sprintf("group%d", i%10)
			model := fmt.Sprintf("model%d", i%5)
			rl.GetRequest(60*time.Second, group, model)
			i++
		}
	})
}
