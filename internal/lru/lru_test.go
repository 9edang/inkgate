package lru

import (
	"strconv"
	"testing"
)

func TestNewLruCache(t *testing.T) {
	_, err := NewLruCache(0, 0)
	if err != ErrInvalidMaxSize {
		t.Errorf("expecting ErrInvalidMaxSize, got %v", err)
	}

	_, err = NewLruCache(1, 0)
	if err != ErrInvalidMaxItems {
		t.Errorf("expecting ErrInvalidMaxItems, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	c, _ := NewLruCache(10000, 3)
	c.Set("a", []byte("A"))
	c.Set("b", []byte("B"))
	c.Set("c", []byte("C"))

	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Errorf("expecting key a to be deleted")
	}
}

func TestGet(t *testing.T) {
	c, _ := NewLruCache(10000, 3)

	for i := 1; i <= 3; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte(v))
	}

	for i := 1; i <= 3; i++ {
		v := strconv.Itoa(i)
		if _, ok := c.Get(v); !ok {
			t.Errorf("expecting key %s to exist", v)
		}
	}

	if _, ok := c.Get("4"); ok {
		t.Errorf("expecting key 4 to not exist")
	}
}

func TestSet_invalidSize(t *testing.T) {
	c, _ := NewLruCache(10, 10)
	c.Set("too big", []byte("too big to fit the cache size"))

	if _, ok := c.Get("too big"); ok {
		t.Errorf("expecting key a to not exist")
	}
}

func TestSet_evictDueToItems(t *testing.T) {
	c, _ := NewLruCache(10000, 10)

	for i := 1; i <= 10; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte(v))
	}

	c.Set("11", []byte("11"))

	if _, ok := c.Get("1"); ok {
		t.Errorf("expecting key 1 to be evicted")
	}

	c.Get("2")

	c.Set("12", []byte("12"))
	if _, ok := c.Get("2"); !ok {
		t.Errorf("expecting key 2 to exist")
	}
}

func TestSet_evictDueToSize(t *testing.T) {
	c, _ := NewLruCache(10, 100)

	for i := 1; i <= 10; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte("v"))
	}

	c.Set("11", []byte("v"))

	if _, ok := c.Get("1"); ok {
		t.Errorf("expecting key 1 to be evicted")
	}

	c.Set("12", []byte("vvv"))
	for i := 2; i <= 4; i++ {
		v := strconv.Itoa(i)
		if _, ok := c.Get(v); ok {
			t.Errorf("expecting key %s to be evicted", v)
		}
	}
}

func TestSet_moveToFront(t *testing.T) {
	c, _ := NewLruCache(10000, 10)

	for i := 1; i <= 10; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte(v))
	}

	c.Set("1", []byte("1"))
	c.Set("11", []byte("11"))

	if _, ok := c.Get("1"); !ok {
		t.Errorf("expecting key 1 to exist")
	}
	if _, ok := c.Get("2"); ok {
		t.Errorf("expecting key 2 to be evicted")
	}
}
