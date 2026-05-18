package shm

import (
	"context"
	"log"
	"sync"
	"time"
)

type CategoryLookup interface {
	ServiceCategory(ctx context.Context, userServiceID int) (string, error)
}

type CachedCategoryLookup struct {
	inner CategoryLookup
	ttl   time.Duration
	mu    sync.RWMutex
	items map[int]cacheItem
}

type cacheItem struct {
	category  string
	expiresAt time.Time
}

func NewCachedCategoryLookup(inner CategoryLookup, ttl time.Duration) *CachedCategoryLookup {
	return &CachedCategoryLookup{
		inner: inner,
		ttl:   ttl,
		items: map[int]cacheItem{},
	}
}

func (c *CachedCategoryLookup) ServiceCategory(ctx context.Context, userServiceID int) (string, error) {
	if c.ttl > 0 {
		now := time.Now()
		c.mu.RLock()
		item, ok := c.items[userServiceID]
		c.mu.RUnlock()
		if ok && now.Before(item.expiresAt) {
			log.Printf("SHM service category cache hit user_service_id=%d category=%s", userServiceID, item.category)
			return item.category, nil
		}
	}

	category, err := c.inner.ServiceCategory(ctx, userServiceID)
	if err != nil {
		return "", err
	}

	if c.ttl > 0 {
		c.mu.Lock()
		c.items[userServiceID] = cacheItem{
			category:  category,
			expiresAt: time.Now().Add(c.ttl),
		}
		c.mu.Unlock()
		log.Printf("SHM service category cached user_service_id=%d category=%s ttl=%s", userServiceID, category, c.ttl)
	}

	return category, nil
}
