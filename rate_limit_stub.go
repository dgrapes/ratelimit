package main

import (
	"time"
)

type _NoRateLimiter struct {
	limitPeriod time.Duration // 1 min, 1 sec, etc. Не может меняться в процессе работы
}

func NewNoRateLimiter(limitPeriod time.Duration) RateLimiter {
	return &_NoRateLimiter{
		limitPeriod: limitPeriod,
	}
}

func (r *_NoRateLimiter) LimitPeriod() time.Duration {
	return r.limitPeriod
}

func (r *_NoRateLimiter) Check(_ string, _ string) (allow bool, limit int, remain int) {
	return true, Unlimited, Unlimited
}

func (r *_NoRateLimiter) DeleteExpired(_ int) (nbRecordsDeleted int) {
	return 0
}
