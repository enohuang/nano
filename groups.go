package nano

import (
	"fmt"
	"sync"

	"gnano/internal/env"
	"gnano/internal/log"
)

type Groups struct {
	mu     sync.RWMutex
	groups map[uint64]*Group
}

func NewGroups() *Groups {
	return &Groups{mu: sync.RWMutex{}, groups: map[uint64]*Group{}}
}

// Member returns specified roomNo's Group
func (cs *Groups) Member(key uint64) (*Group, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	g, ok := cs.groups[key]
	if ok {
		return g, nil
	}

	return nil, ErrMemberNotFound
}

// Members returns all member's roomNo in current group
func (c *Groups) Members() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var members []uint64
	for k, _ := range c.groups {
		members = append(members, k)
	}
	return members
}

// Contains check whether a roomNo is contained in current group or not
func (cs *Groups) Contains(key uint64) bool {
	_, err := cs.Member(key)
	return err == nil
}

// Add add group to groups
func (cs *Groups) Add(key uint64, group *Group) error {

	if env.Debug {
		log.Println(fmt.Sprintf("Add group to groups , key=%d", key))
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, ok := cs.groups[key]
	if ok {
		return ErrSessionDuplication
	}

	cs.groups[key] = group
	return nil
}

// Leave remove specified DeskNo related group from groups
func (c *Groups) Leave(key uint64) error {

	if env.Debug {
		log.Println(fmt.Sprintf("Remove group from groups , key=%d", key))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.groups, key)
	return nil
}

// LeaveAll clear all group in the groups
func (c *Groups) LeaveAll() error {

	c.mu.Lock()
	defer c.mu.Unlock()
	c.groups = make(map[uint64]*Group)
	return nil
}

// Count get current member amount in the groups
func (c *Groups) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.groups)
}
