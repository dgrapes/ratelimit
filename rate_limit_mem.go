package main

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type _Limit struct {
	until  time.Time // до какого времени действует ограничение
	remain int       // сколько еще остается вызовов
	limit  int
}

type _MemoryRateLimiter struct {
	limitPeriod time.Duration // 1 min, 1 sec, etc. Не может меняться в процессе работы

	mu           sync.Mutex
	baseLimit    int               // самый базовый лимит
	groupLimits  map[string]int    // NotUsed => baseLimit
	sellerLimits map[string]int    // key="seller|group" или "seller". Тут не может быть NotUsed (такие ключи удаляются)
	current      map[string]_Limit // текущие лимиты. key=???
}

func NewInMemoryRateLimiter(baseLimit int, limitPeriod time.Duration) IConfigurableRateLimiter {
	if baseLimit < 0 { // 0 is ok
		panic("negative rate limit value")
	}
	if limitPeriod <= 0 {
		panic("invalid rate limit period")
	}

	res := &_MemoryRateLimiter{
		limitPeriod:  limitPeriod,
		baseLimit:    baseLimit,
		groupLimits:  make(map[string]int),
		sellerLimits: make(map[string]int),
		current:      make(map[string]_Limit),
	}

	return res
}

func (r *_MemoryRateLimiter) SetAccountingGroups(groups []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.groupLimits = make(map[string]int)
	for _, g := range groups {
		r.groupLimits[g] = NotUsed
	}

	r.sellerLimits = make(map[string]int)
}

func (r *_MemoryRateLimiter) LimitPeriod() time.Duration {
	return r.limitPeriod
}

func (r *_MemoryRateLimiter) SetLimit(seller, group string, limit int) error {

	if limit < NotUsed {
		return errors.New("invalid limit")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if group != "" {
		if _, ok := r.groupLimits[group]; !ok {
			return errors.New("group does not exist")
		}
	}

	if seller != "" {
		k := seller
		if group != "" {
			k += "|" + group
		}
		if limit == NotUsed {
			delete(r.sellerLimits, k)
		} else {
			r.sellerLimits[k] = limit
		}
	} else {
		if group != "" {
			r.groupLimits[group] = limit
		} else {
			if limit == NotUsed {
				return errors.New("cant delete base limit")
			}
			r.baseLimit = limit
		}
	}
	return nil
}

func (r *_MemoryRateLimiter) RemoveSellerLimits(seller string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if seller == "" {
		r.sellerLimits = make(map[string]int)
	} else {
		prefix := seller + "|"
		for k, _ := range r.sellerLimits {
			if strings.HasPrefix(k, prefix) {
				delete(r.sellerLimits, k)
			}
		}
	}
}
func (r *_MemoryRateLimiter) RemoveAllOverrides() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sellerLimits = make(map[string]int)
	for k := range r.groupLimits {
		r.groupLimits[k] = NotUsed
	}
}

/*
	seller указывается всегда (мы же для каждого делаем лимитирование)
	group: опциональна. Позволяет делать раздельные лимиты.
			В рамках группы лимит общий.
			Группа может быть на один метод или на группу, например, "post"
			Если группа указана, она должна быть известна.
		- если не указан,	limit = first("seller|", baseLimit)
		- если указан		limit = first("seller|method", "seller|", "|method", baseLimit)
*/
func (r *_MemoryRateLimiter) Check(seller, group string) (allow bool, limit, remain int) {

	if seller == "" {
		return false, 0, 0
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	// существует ли такая учетная группа или считаем его в дефолтную группу?
	if group != "" {
		if _, ok := r.groupLimits[group]; !ok {
			group = "" // такая группа не существует, ведем учет через общую группу
		}
	}

	var k string
	if group == "" {
		k = seller
	} else {
		k = seller + "|" + group
	}

	l, exists := r.current[k]
	if exists && now.Before(l.until) { // если действующий лимит есть и он актуален
		allow = l.remain > 0
		if allow {
			l.remain--
			r.current[k] = l
		}
		return allow, l.limit, l.remain
	} else {
		switch limit = r.getLimitUnsafe(seller, group); limit {
		case Unlimited:
			if exists { // удалим из бд устаревший лимит
				delete(r.current, k)
			}
			return true, Unlimited, Unlimited
		case 0:
			if exists { // удалим из бд устаревший лимит
				delete(r.current, k)
			}
			return false, 0, 0
		default:
			allow = true
			remain = limit - 1 // помним, что limit > 0; это уже проверили выше
			r.current[k] = _Limit{
				until:  now.Add(r.limitPeriod),
				remain: remain,
				limit:  limit,
			}
			return
		}
	}
}

// seller может быть пустой
// группа может быть пустая и может быть несуществующая.
func (r *_MemoryRateLimiter) getLimitUnsafe(seller, group string) int {

	if seller != "" {
		if group != "" {
			// я не проверяю группу на каноничность потому что иначе такого ключа просто не будет в словаре
			k := seller + "|" + group
			if v, ok := r.sellerLimits[k]; ok { // NotUsed быть не может
				return v
			}
		}
		// если мы тут, значит группа не указана или для нее нет значения
		if v, ok := r.sellerLimits[seller]; ok {
			return v
		}
	}
	// тут seller пустой или не найден
	if group != "" {
		v, ok := r.groupLimits[group] // здесь NotUsed уже может быть
		if ok && v != NotUsed {
			return v
		}
	}
	return r.baseLimit
}

func (r *_MemoryRateLimiter) DeleteExpired(maxRecords int) (nbRecordsDeleted int) {

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for k, v := range r.current {
		if now.After(v.until) {
			nbRecordsDeleted++
			delete(r.current, k)

			if maxRecords > 0 && nbRecordsDeleted == maxRecords {
				break
			}
		}
	}

	return
}
