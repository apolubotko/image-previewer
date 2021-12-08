package proxy

import (
	"sync"
)

type lruCache struct {
	Cache
	mu       sync.Mutex
	capacity int
	queue    List
	items    map[Key]*cacheItem
}

type cacheItem struct {
	key   string
	value interface{}
}

func NewCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		queue:    NewList(),
		items:    make(map[Key]*cacheItem, capacity),
	}
}

func (l *lruCache) Set(key Key, value interface{}) bool {
	var listItem *ListItem
	var cItem *cacheItem

	// If element exists
	l.mu.Lock()
	defer l.mu.Unlock()
	cItem, exist := l.items[key]
	if exist {
		listItem = cItem.value.(*ListItem)
		listItem.Value = value
		l.items[key] = cItem
		l.queue.MoveToFront(listItem)

		return true
	}

	// If element not exist
	listItem = l.queue.PushFront(value)
	listItem.Key = key
	cItem = &cacheItem{
		key:   string(key),
		value: listItem,
	}
	l.items[key] = cItem

	if l.queue.Len() > l.capacity {
		tail := l.queue.Back()
		l.queue.Remove(tail)
		delete(l.items, tail.Key)
	}

	return false
}

func (l *lruCache) Get(key Key) (interface{}, bool) {
	var listItem *ListItem

	// If element exists
	if cItem, exist := l.items[key]; exist {
		listItem = cItem.value.(*ListItem)
		l.queue.MoveToFront(listItem)

		return listItem.Value, true
	}

	// If element doesn't exist
	return nil, false
}

func (l *lruCache) Clear() {
	l.queue = NewList()
	l.items = make(map[Key]*cacheItem, l.capacity)
}
