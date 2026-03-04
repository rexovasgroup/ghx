package set

var exists = struct{}{}

type stringSet struct {
	v []string
	m map[string]struct{}
}

// NewStringSet creates a new set of strings.
func NewStringSet() *stringSet {
	s := &stringSet{}
	s.m = make(map[string]struct{})
	s.v = []string{}
	return s
}

// Add inserts a value into the set.
func (s *stringSet) Add(value string) {
	if s.Contains(value) {
		return
	}
	s.m[value] = exists
	s.v = append(s.v, value)
}

// AddValues inserts multiple values into the set.
func (s *stringSet) AddValues(values []string) {
	for _, v := range values {
		s.Add(v)
	}
}

// Remove deletes a value from the set.
func (s *stringSet) Remove(value string) {
	if !s.Contains(value) {
		return
	}
	delete(s.m, value)
	s.v = sliceWithout(s.v, value)
}

func sliceWithout(s []string, v string) []string {
	idx := -1
	for i, item := range s {
		if item == v {
			idx = i
			break
		}
	}
	if idx < 0 {
		return s
	}
	return append(s[:idx], s[idx+1:]...)
}

// RemoveValues deletes multiple values from the set.
func (s *stringSet) RemoveValues(values []string) {
	for _, v := range values {
		s.Remove(v)
	}
}

// Contains reports whether the set contains the given value.
func (s *stringSet) Contains(value string) bool {
	_, c := s.m[value]
	return c
}

// Len returns the number of elements in stringSet.
func (s *stringSet) Len() int {
	return len(s.m)
}

// ToSlice performs the ToSlice operation on stringSet.
func (s *stringSet) ToSlice() []string {
	return s.v
}

// Equal reports whether two sets contain the same elements.
func (s1 *stringSet) Equal(s2 *stringSet) bool {
	if s1.Len() != s2.Len() {
		return false
	}
	isEqual := true
	for _, v := range s1.v {
		if !s2.Contains(v) {
			isEqual = false
			break
		}
	}
	return isEqual
}
