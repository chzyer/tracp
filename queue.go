package tracp

import (
	"container/heap"
	"time"

	"github.com/google/gopacket"
)

type Queue []*QueueItem

func (q Queue) Len() int {
	return len(q)
}

func (q Queue) Less(i, j int) bool {
	return q[i].sendTime.Before(q[j].sendTime)
}

func (q Queue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *Queue) SendInTime(p gopacket.Packet, send time.Time) {
	item := &QueueItem{
		packet:   p,
		sendTime: send,
	}
	heap.Push(q, item)
	heap.Fix(q, item.index)
}

func (q *Queue) GetLastest() (gopacket.Packet, time.Time) {
	if q.Len() == 0 {
		return nil, time.Time{}
	}

	now := time.Now()
	want := (*q)[0].sendTime
	if now.After(want) {
		item := heap.Pop(q).(*QueueItem)
		return item.packet, item.sendTime
	}
	return nil, want
}

func (q *Queue) Push(obj interface{}) {
	n := len(*q)
	item := obj.(*QueueItem)
	item.index = n
	*q = append(*q, item)
}

func (q *Queue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*q = old[0 : n-1]
	return item
}

type QueueItem struct {
	packet   gopacket.Packet
	sendTime time.Time
	index    int
}
