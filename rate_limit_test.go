package main

import (
	"fmt"
	"testing"
	"time"
)

func assert(cond bool, msg string, t *testing.T) {
	if !cond {
		t.Fatal(msg)
	}
}

func TestInMemoryRateLimiter(t *testing.T) {
	rl := NewInMemoryRateLimiter(5, time.Second) // 5 запросов в сек
	for i := 0; i < 5; i++ {
		ok, limit, remain := rl.Check("seller", "")
		if !ok {
			t.Fatalf("Expected ALLOW in iter %d", i)
		}
		if limit != 5 {
			t.Fatalf("wrong limit %d", limit)
		}
		if remain != 4-i {
			t.Fatalf("wrong remain %d", remain)
		}
	}
	for i := 0; i < 50; i++ {
		ok, limit, remain := rl.Check("seller", "")
		if ok {
			t.Fatalf("Expected DISALLOW in iter %d", i)
		}
		if limit != 5 {
			t.Fatalf("wrong limit %d", limit)
		}
		if remain != 0 {
			t.Fatalf("wrong remain %d", remain)
		}
	}
	for i := 0; i < 5; i++ {
		ok, limit, remain := rl.Check(fmt.Sprintf("seller%d", i), "")
		if !ok {
			t.Fatalf("Expected ALLOW for seller #%d", i)
		}
		if limit != 5 {
			t.Fatalf("wrong limit %d", limit)
		}
		if remain != 4 {
			t.Fatalf("wrong remain %d", remain)
		}
	}
	time.Sleep(1100 * time.Millisecond)
	for i := 0; i < 5; i++ {
		ok, _, _ := rl.Check("seller", "")
		if !ok {
			t.Fatalf("Expected ALLOW in iter %d", i)
		}
	}
	for i := 0; i < 50; i++ {
		ok, _, _ := rl.Check("seller", "")
		if ok {
			t.Fatalf("Expected DISALLOW in iter %d", i)
		}
	}
	for i := 0; i < 5; i++ {
		ok, _, _ := rl.Check(fmt.Sprintf("seller%d", i), "")
		if !ok {
			t.Fatalf("Expected ALLOW for seller #%d", i)
		}
	}
}

// проверяет, что мы можем отправить nb запросов, а также после их отправки будет оставаться remainder и лимит
func runChecks(r RateLimiter, seller, method string, nb int, remainder, limit int, t *testing.T) {
	var ok bool
	var rem, lim int
	for i := 0; i < nb; i++ {
		ok, lim, rem = r.Check(seller, method)
		if !ok {
			t.Fatalf("Expected ALLOW for %s/%s[%d]", seller, method, i)
		}
	}
	if rem != remainder {
		t.Fatalf("Expected remainder=%d (not %d) for %s/%s", remainder, rem, seller, method)
	}
	if lim != limit {
		t.Fatalf("Expected limit=%d (not %d) for %s/%s", limit, lim, seller, method)
	}
}
func maxGood(r RateLimiter, seller, method string, nbGood int, t *testing.T) {
	for i := 0; i < nbGood; i++ {
		ok, _, _ := r.Check(seller, method)
		if !ok {
			t.Fatalf("Expected ALLOW for %s/%s[%d]", seller, method, i)
		}
	}
	ok, _, _ := r.Check(seller, method)
	if ok {
		t.Fatalf("Expected DISALLOW for %s/%s", seller, method)
	}
}

func expectDisallow(r RateLimiter, seller, method string, nb int, t *testing.T) {
	for i := 0; i < nb; i++ {
		ok, _, _ := r.Check(seller, method)
		if ok {
			t.Fatalf("Expected DISALLOW for %s/%s[%d]", seller, method, i)
		}
	}
}

func TestInMemoryRateLimiterConfig(t *testing.T) {
	const (
		period     = 100 * time.Millisecond
		waitPeriod = 110 * time.Millisecond
	)

	rl := NewInMemoryRateLimiter(5, period) // 5 запросов в сек
	rl.SetAccountingGroups([]string{"a", "b"})

	for i := 0; i < 2; i++ {
		runChecks(rl, "s1", "", 5, 0, 5, t)
		runChecks(rl, "s1", "a", 5, 0, 5, t)
		runChecks(rl, "s1", "b", 5, 0, 5, t)
		expectDisallow(rl, "s1", "", 10, t)
		expectDisallow(rl, "s1", "a", 10, t)
		expectDisallow(rl, "s1", "b", 10, t)
		expectDisallow(rl, "s1", "c", 10, t)

		runChecks(rl, "s2", "", 5, 0, 5, t)
		runChecks(rl, "s2", "a", 5, 0, 5, t)
		runChecks(rl, "s2", "b", 5, 0, 5, t)
		expectDisallow(rl, "s2", "", 10, t)
		expectDisallow(rl, "s2", "a", 10, t)
		expectDisallow(rl, "s2", "b", 10, t)
		expectDisallow(rl, "s2", "c", 10, t)

		time.Sleep(waitPeriod)
	}

	rl.SetLimit("", "a", 3)
	runChecks(rl, "s1", "", 3, 2, 5, t)
	runChecks(rl, "s1", "a", 3, 0, 3, t)
	runChecks(rl, "s1", "b", 3, 2, 5, t)
	runChecks(rl, "s1", "c", 2, 0, 5, t) // "c" суммируется с ""

	time.Sleep(waitPeriod)
	rl.RemoveAllOverrides()
	rl.SetLimit("s1", "", 3)
	rl.SetLimit("s1", "a", 5)
	rl.SetLimit("s2", "b", 3)
	maxGood(rl, "s1", "", 3, t)
	maxGood(rl, "s1", "a", 5, t)
	maxGood(rl, "s1", "b", 3, t)
	maxGood(rl, "s2", "", 5, t)
	maxGood(rl, "s2", "a", 5, t)
	maxGood(rl, "s2", "b", 3, t)

	time.Sleep(waitPeriod)
	rl.RemoveAllOverrides()
	rl.SetLimit("", "", 10)
	rl.SetLimit("", "a", 20)
	rl.SetLimit("s1", "", 30)
	rl.SetLimit("s2", "b", 40)

	maxGood(rl, "s1", "", 30, t)
	maxGood(rl, "s1", "a", 30, t)
	maxGood(rl, "s1", "b", 30, t)
	maxGood(rl, "s2", "", 10, t)
	maxGood(rl, "s2", "a", 20, t)
	maxGood(rl, "s2", "b", 40, t)
}

func TestGetPeriod(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, time.Second) // 1 запросов в сек

	p := rl.LimitPeriod()
	assert(p.Milliseconds() == 1000, "", t)
}

func BenchmarkInMemoryRateLimiter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rl := NewInMemoryRateLimiter(1, time.Second) // 5 запросов в сек
		rl.Check("seller", "")
	}
}

func TestNoRateLimiter(t *testing.T) {
	rl := NewNoRateLimiter(time.Second)
	runChecks(rl, "", "", 1000, Unlimited, Unlimited, t)
}

func TestConfigurationLoad(t *testing.T) {
	d := []byte(`
	{
		"groups":["a", "b"],
		"limits":[
			{"seller": "", "group": "", "value": 20},
			{"seller": "s1", "group": "", "value": 10},
			{"seller": "s1", "group": "a", "value": 15}
		]
	}
	`)
	rl := NewInMemoryRateLimiter(1, time.Second) // 5 запросов в сек

	err := ConfigureRateLimiter(rl, d)
	if err != nil {
		t.Fatal(err)
	}

	maxGood(rl, "s", "", 20, t)
	maxGood(rl, "s", "a", 20, t)
	maxGood(rl, "s", "b", 20, t)
	maxGood(rl, "s", "c", 0, t) // потому что это в общую группу коммитится, а там лимит уже выбран

	maxGood(rl, "s1", "", 10, t)
	maxGood(rl, "s1", "a", 15, t)
	maxGood(rl, "s1", "b", 10, t)
	maxGood(rl, "s1", "c", 0, t) // потому что это в общую группу коммитится, а там лимит уже выбран
}

/*
func Test__InMemoryRateLimiter_Check(t *testing.T) {
	type fields struct {
		limitValue  int
		limitPeriod time.Duration
	}
	type args struct {
		sellerId string
		method   string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantAllow  bool
		wantLimit  int
		wantRemain int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &_MemoryRateLimiter{
				limitValue:  tt.fields.limitValue,
				limitPeriod: tt.fields.limitPeriod,
				limits:      tt.fields.limits,
			}
			gotAllow, gotLimit, gotRemain := r.Check(tt.args.sellerId, tt.args.method)
			if gotAllow != tt.wantAllow {
				t.Errorf("Check() gotAllow = %v, want %v", gotAllow, tt.wantAllow)
			}
			if gotLimit != tt.wantLimit {
				t.Errorf("Check() gotLimit = %v, want %v", gotLimit, tt.wantLimit)
			}
			if gotRemain != tt.wantRemain {
				t.Errorf("Check() gotRemain = %v, want %v", gotRemain, tt.wantRemain)
			}
		})
	}
}
*/
