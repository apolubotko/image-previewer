package cache

type List interface {
	Len() int
	Front() *ListItem
	Back() *ListItem
	PushFront(v interface{}) *ListItem
	PushBack(v interface{}) *ListItem
	Remove(i *ListItem)
	MoveToFront(i *ListItem)
}

type ListItem struct {
	Value interface{}
	Key   Key
	Next  *ListItem
	Prev  *ListItem
}

type list struct {
	List
	items map[*ListItem]struct{}
	head  *ListItem
	tail  *ListItem
}

func (l *list) Len() int {
	return len(l.items)
}

func (l *list) Front() *ListItem {
	if l.items == nil {
		return nil
	}
	return l.head
}

func (l *list) Back() *ListItem {
	if l.items == nil {
		return nil
	}
	return l.tail
}

func (l *list) PushFront(v interface{}) *ListItem {
	elem := &ListItem{Value: v}

	if l.items == nil {
		l.head = elem
		l.tail = elem
		l.items = make(map[*ListItem]struct{})
		l.items[elem] = struct{}{}
		return elem
	}
	elem.Next = l.head
	l.head.Prev = elem
	l.head = elem
	l.items[elem] = struct{}{}

	return elem
}

func (l *list) PushBack(v interface{}) *ListItem {
	elem := &ListItem{
		Value: v,
	}

	if l.items == nil {
		l.head = elem
		l.tail = elem
		l.items = make(map[*ListItem]struct{})
		l.items[elem] = struct{}{}

		return elem
	}
	elem.Prev = l.tail
	l.tail.Next = elem
	l.tail = elem
	l.items[elem] = struct{}{}

	return elem
}

func (l *list) Remove(i *ListItem) {
	switch i {
	case l.head:
		l.head = i.Next
		l.head.Prev = nil
	case l.tail:
		l.tail = i.Prev
		l.tail.Next = nil
	default:
		i.Prev.Next = i.Next
		i.Next.Prev = i.Prev
	}
	delete(l.items, i)
}

func (l *list) MoveToFront(i *ListItem) {
	if _, ok := l.items[i]; !ok {
		return
	}
	if i == l.head {
		return
	}
	i.Prev.Next = i.Next
	if i != l.tail {
		i.Next.Prev = i.Prev
	}
	i.Next = l.head
	i.Prev = nil
	l.head.Prev = i
	l.head = i
	if i.Next.Next == nil {
		l.tail = i.Next
	}
}

func NewList() List {
	return new(list)
}
