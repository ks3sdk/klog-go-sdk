package service

import "fmt"

// A SettableBool provides a boolean value which includes the state if
// the value was set or unset.  The set state is in addition to the value's
// value(true|false)
type SettableBool struct {
	value bool
	set   bool
}

// SetBool returns a SettableBool with a value set
func SetBool(value bool) SettableBool {
	return SettableBool{value: value, set: true}
}

// Get returns the value. Will always be false if the SettableBool was not set.
func (b *SettableBool) Get() bool {
	if !b.set {
		return false
	}
	return b.value
}

// Set sets the value and updates the state that the value has been set.
func (b *SettableBool) Set(value bool) {
	b.value = value
	b.set = true
}

// IsSet returns if the value has been set
func (b *SettableBool) IsSet() bool {
	return b.set
}

// Reset resets the state and value of the SettableBool to its initial default
// state of not set and zero value.
func (b *SettableBool) Reset() {
	b.value = false
	b.set = false
}

// String returns the string representation of the value if set. Zero if not set.
func (b *SettableBool) String() string {
	return fmt.Sprintf("%t", b.Get())
}

// GoString returns the string representation of the SettableBool value and state
func (b *SettableBool) GoString() string {
	return fmt.Sprintf("Bool{value:%t, set:%t}", b.value, b.set)
}
