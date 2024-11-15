package state

import (
	"fmt"
	"reflect"
	"submesh/submesh/types"
	"sync"
)

type HistoricalWithLastByPK[T any] struct {
	all    []types.ParsedMessage[T]
	lastBy map[string]*types.ParsedMessage[T]
	lock   sync.RWMutex
	Limit  int
}

const defaultLimit = 5000

func NewHistoricalWithLastByPK[T any]() HistoricalWithLastByPK[T] {
	return HistoricalWithLastByPK[T]{
		all:    make([]types.ParsedMessage[T], 0),
		lastBy: make(map[string]*types.ParsedMessage[T]),
		Limit:  defaultLimit,
	}
}

func (h *HistoricalWithLastByPK[T]) Add(t types.ParsedMessage[T], pks ...string) {
	h.lock.Lock()
	defer h.lock.Unlock()

	//prepend to all
	h.all = append([]types.ParsedMessage[T]{t}, h.all...)
	if h.Limit > 0 && len(h.all) > h.Limit {
		h.all = h.all[:h.Limit]
	}

	for _, pk := range pks {
		h.lastBy[pk] = &t
	}
}

func (h *HistoricalWithLastByPK[T]) All() []types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.all
}

func (h *HistoricalWithLastByPK[T]) LastBy(pk string) *types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.lastBy[pk]
}
func (h *HistoricalWithLastByPK[T]) LastByProperty(pk string, prop string) *types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	for _, prk := range h.all {
		val := reflect.ValueOf(prk).FieldByName(pk)
		if prop == coalesceReflectValueToString(val) {
			return &prk
		}
	}
	return nil
}
func coalesceReflectValueToString(val reflect.Value) string {
	if val.CanUint() {
		return fmt.Sprintf("%d", val.Uint())
	} else if val.CanFloat() {
		return fmt.Sprintf("%f", val.Float())
	} else if val.CanInt() {
		return fmt.Sprintf("%d", val.Int())
	} else {
		return val.String()
	}
}

// All items, but the last added, by Property
func (h *HistoricalWithLastByPK[T]) OnlyMostRecentByPropertyString(pk string) []types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	var onlyLast []types.ParsedMessage[T]
	added := map[string]bool{}

	for _, item := range h.all {
		name := coalesceReflectValueToString(reflect.ValueOf(item).FieldByName(pk))
		if !added[name] {
			onlyLast = append(onlyLast, item)
			added[name] = true
		}
	}

	return onlyLast
}

// All items, but the last added, by Property
func (h *HistoricalWithLastByPK[T]) OnlyMostRecentByUnderlyingPropertyString(pk string) []types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	var onlyLast []types.ParsedMessage[T]
	added := map[string]bool{}

	for _, item := range h.all {
		name := coalesceReflectValueToString(reflect.ValueOf(item.Underlying).FieldByName(pk))
		if !added[name] {
			onlyLast = append(onlyLast, item)
			added[name] = true
		}
	}

	return onlyLast
}

// All items, but filtered, by Property
func (h *HistoricalWithLastByPK[T]) FilteredByString(property string, value string) []types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	var onlyLast []types.ParsedMessage[T]

	for _, item := range h.all {
		name := coalesceReflectValueToString(reflect.ValueOf(item).FieldByName(property))
		if name == value {
			onlyLast = append(onlyLast, item)
		}
	}

	return onlyLast
}

// All items, but filtered, by Property
func (h *HistoricalWithLastByPK[T]) FilteredByUnderlyingString(property string, value string) []types.ParsedMessage[T] {
	h.lock.RLock()
	defer h.lock.RUnlock()
	var onlyLast []types.ParsedMessage[T]

	for _, item := range h.all {
		name := coalesceReflectValueToString(reflect.ValueOf(item.Underlying).FieldByName(property))
		if name == value {
			onlyLast = append(onlyLast, item)
		}
	}

	return onlyLast
}
