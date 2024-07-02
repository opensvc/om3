package prioqueue

import (
	"container/heap"
)

type Interface interface {
	Before(Interface) bool
}

const shrinkMinCap = 1000
const shrinkNewSizeFactor = 2
const shrinkCapLenFactorCondition = 4

type Queue struct {
	queue heap.Interface
}

func New() *Queue {
	pq := &Queue{}
	pq.queue = newHeapMemory(
		shrinkMinCap,
		shrinkNewSizeFactor,
		shrinkCapLenFactorCondition,
	)
	return pq
}

func (pq *Queue) Len() int {
	return pq.queue.Len()
}

func (pq *Queue) Push(something Interface) {
	heap.Push(pq.queue, something)
}

func (pq *Queue) Pop() Interface {

	if pq.queue.Len() <= 0 {
		return nil
	}
	r := heap.Pop(pq.queue)
	return r.(Interface)
}

type heapMemory struct {
	slice                       internalSlice
	ShrinkMinCap                int
	ShrinkNewSizeFactor         int
	ShrinkCapLenFactorCondition int
}

func newHeapMemory(shrinkMinCap, shrinkNewSizeFactor, shrinkCapLenFactorCondition int) *heapMemory {
	return &heapMemory{
		slice:                       make(internalSlice, 0),
		ShrinkMinCap:                shrinkMinCap,
		ShrinkNewSizeFactor:         shrinkNewSizeFactor,
		ShrinkCapLenFactorCondition: shrinkCapLenFactorCondition,
	}
}

type internalSlice []Interface

func (pq *heapMemory) Len() int { return len(pq.slice) }

func (pq *heapMemory) Less(i, j int) bool {
	return pq.slice[i].Before(pq.slice[j])
}

func (pq *heapMemory) Swap(i, j int) {
	pq.slice[i], pq.slice[j] = pq.slice[j], pq.slice[i]
}

func (pq *heapMemory) Push(x interface{}) {
	pq.slice = append(pq.slice, x.(Interface))
}

func (pq *heapMemory) Pop() interface{} {
	old, n := pq.shrinkIfNeeded()
	item := (*old)[n-1]
	pq.slice = (*old)[0 : n-1]
	return item
}

func (pq *heapMemory) shrinkIfNeeded() (*internalSlice, int) {
	l, c := len(pq.slice), cap(pq.slice)
	if cap(pq.slice) > pq.ShrinkMinCap && c/l > pq.ShrinkCapLenFactorCondition {
		newSlice := make(internalSlice, pq.ShrinkNewSizeFactor*l, pq.ShrinkNewSizeFactor*l)
		for i, v := range pq.slice {
			newSlice[i] = v
		}
		return &newSlice, l
	}
	return &pq.slice, l
}
