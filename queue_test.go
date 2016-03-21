package tracp

import (
	"testing"
	"time"
)

func TestQueue(t *testing.T) {
	q := Queue{}
	test := []time.Time{
		time.Now().Add(-3 * time.Second),
		time.Now().Add(-time.Second),
		time.Now().Add(-2 * time.Second),
	}
	for _, tt := range test {
		q.SendInTime(nil, tt)
	}
	var now time.Time
	for i := 0; ; i++ {
		_, rt := q.GetLastest()
		if rt.IsZero() {
			break
		}
		if rt.After(now) {
			now = rt
			continue
		}
		t.Fatal()
	}
}
