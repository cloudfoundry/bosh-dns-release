package tracker

import "sync"

type link struct {
	name       string
	prev, next *link
}

func (l *link) remove() {
	l.prev.next = l.next
	l.next.prev = l.prev
}

func (l *link) append(after *link) {
	after.next = l.next
	l.next.prev = after
	l.next = after
	after.prev = l
}

type PriorityLimitedTranscript struct {
	names      map[string]*link
	tail, head *link

	length, cap uint
	mutex       *sync.RWMutex
}

func NewPriorityLimitedTranscript(cap uint) *PriorityLimitedTranscript {
	head := link{}
	tail := link{next: &head}
	head.prev = &tail

	return &PriorityLimitedTranscript{
		names: make(map[string]*link),
		cap:   cap,
		mutex: &sync.RWMutex{},
		head:  &head,
		tail:  &tail,
	}
}

func (t *PriorityLimitedTranscript) oldest() *link {
	return t.tail.next
}

func (t *PriorityLimitedTranscript) newest() *link {
	return t.head.prev
}

func (t *PriorityLimitedTranscript) Touch(s string) string {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	var that *link
	var ok bool
	removed := ""

	if that, ok = t.names[s]; ok {
		that.remove()

		t.length--
	} else {
		that = &link{
			name: s,
		}

		t.names[s] = that
	}

	if t.length >= t.cap {
		oldest := t.oldest()
		oldest.remove()
		delete(t.names, oldest.name)

		t.length--
		removed = oldest.name
	}

	t.newest().append(that)

	t.length++
	return removed
}

func (t *PriorityLimitedTranscript) Registry() []string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	r := make([]string, len(t.names))
	i := 0
	for s := range t.names {
		r[i] = s
		i++
	}
	return r
}
