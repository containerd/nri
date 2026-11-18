/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"os"
	"slices"
)

// NewString creates an OptionalString from the given string value.
func NewString(v string) *OptionalString {
	return &OptionalString{Value: v}
}

// StringPtr creates an OptionalString from a pointer, returning nil if the pointer is nil.
func StringPtr(v *string) *OptionalString {
	if v == nil {
		return nil
	}
	return &OptionalString{Value: *v}
}

// Copy returns a copy of the OptionalString, or nil if it is nil.
func (o *OptionalString) Copy() *OptionalString {
	if o == nil {
		return nil
	}
	return &OptionalString{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalString) Get() *string {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// NewRepeatedString creates an OptionalRepeatedString from the given string slice.
func NewRepeatedString(v []string) *OptionalRepeatedString {
	return &OptionalRepeatedString{Value: slices.Clone(v)}
}

// RepeatedStringPtr creates an OptionalRepeatedString from a pointer to a slice,
// returning nil if the pointer is nil.
func RepeatedStringPtr(v *[]string) *OptionalRepeatedString {
	if v == nil {
		return nil
	}
	return &OptionalRepeatedString{Value: slices.Clone(*v)}
}

// Copy returns a copy of the OptionalRepeatedString, or nil if it is nil.
func (o *OptionalRepeatedString) Copy() *OptionalRepeatedString {
	if o == nil {
		return nil
	}
	return &OptionalRepeatedString{Value: slices.Clone(o.Value)}
}

// Get returns nil if its value is unset or a pointer to a copy of the value.
func (o *OptionalRepeatedString) Get() *[]string {
	if o == nil {
		return nil
	}
	v := slices.Clone(o.Value)
	return &v
}

// NewInt creates an OptionalInt from the given int value.
func NewInt(v int) *OptionalInt {
	return &OptionalInt{Value: int64(v)}
}

// IntPtr creates an OptionalInt from a pointer, returning nil if the pointer is nil.
func IntPtr(v *int) *OptionalInt {
	if v == nil {
		return nil
	}
	return &OptionalInt{Value: int64(*v)}
}

// Copy returns a copy of the OptionalInt, or nil if it is nil.
func (o *OptionalInt) Copy() *OptionalInt {
	if o == nil {
		return nil
	}
	return &OptionalInt{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalInt) Get() *int {
	if o == nil {
		return nil
	}
	v := int(o.Value)
	return &v
}

// NewInt32 creates an OptionalInt32 from the given int32 value.
func NewInt32(v int32) *OptionalInt32 {
	return &OptionalInt32{Value: v}
}

// Int32Ptr creates an OptionalInt32 from a pointer, returning nil if the pointer is nil.
func Int32Ptr(v *int32) *OptionalInt32 {
	if v == nil {
		return nil
	}
	return &OptionalInt32{Value: *v}
}

// Copy returns a copy of the OptionalInt32, or nil if it is nil.
func (o *OptionalInt32) Copy() *OptionalInt32 {
	if o == nil {
		return nil
	}
	return &OptionalInt32{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalInt32) Get() *int32 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// NewUInt32 creates an OptionalUInt32 from the given uint32 value.
func NewUInt32(v uint32) *OptionalUInt32 {
	return &OptionalUInt32{Value: v}
}

// UInt32Ptr creates an OptionalUInt32 from a pointer, returning nil if the pointer is nil.
func UInt32Ptr(v *uint32) *OptionalUInt32 {
	if v == nil {
		return nil
	}
	return &OptionalUInt32{Value: *v}
}

// Copy returns a copy of the OptionalUInt32, or nil if it is nil.
func (o *OptionalUInt32) Copy() *OptionalUInt32 {
	if o == nil {
		return nil
	}
	return &OptionalUInt32{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalUInt32) Get() *uint32 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// NewInt64 creates an OptionalInt64 from the given int64 value.
func NewInt64(v int64) *OptionalInt64 {
	return &OptionalInt64{Value: v}
}

// Int64Ptr creates an OptionalInt64 from a pointer, returning nil if the pointer is nil.
func Int64Ptr(v *int64) *OptionalInt64 {
	if v == nil {
		return nil
	}
	return &OptionalInt64{Value: *v}
}

// Copy returns a copy of the OptionalInt64, or nil if it is nil.
func (o *OptionalInt64) Copy() *OptionalInt64 {
	if o == nil {
		return nil
	}
	return &OptionalInt64{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalInt64) Get() *int64 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// NewUInt64 creates an OptionalUInt64 from the given uint64 value.
func NewUInt64(v uint64) *OptionalUInt64 {
	return &OptionalUInt64{Value: v}
}

// UInt64Ptr creates an OptionalUInt64 from a pointer, returning nil if the pointer is nil.
func UInt64Ptr(v *uint64) *OptionalUInt64 {
	if v == nil {
		return nil
	}
	return &OptionalUInt64{Value: *v}
}

// Copy returns a copy of the OptionalUInt64, or nil if it is nil.
func (o *OptionalUInt64) Copy() *OptionalUInt64 {
	if o == nil {
		return nil
	}
	return &OptionalUInt64{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalUInt64) Get() *uint64 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// NewBool creates an OptionalBool from the given bool value.
func NewBool(v bool) *OptionalBool {
	return &OptionalBool{Value: v}
}

// BoolPtr creates an OptionalBool from a pointer, returning nil if the pointer is nil.
func BoolPtr(v *bool) *OptionalBool {
	if v == nil {
		return nil
	}
	return &OptionalBool{Value: *v}
}

// Copy returns a copy of the OptionalBool, or nil if it is nil.
func (o *OptionalBool) Copy() *OptionalBool {
	if o == nil {
		return nil
	}
	return &OptionalBool{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalBool) Get() *bool {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// NewFileMode creates an OptionalFileMode from the given os.FileMode value.
func NewFileMode(v os.FileMode) *OptionalFileMode {
	return &OptionalFileMode{Value: uint32(v)}
}

// FileModePtr creates an OptionalFileMode from a pointer, returning nil if the pointer is nil.
func FileModePtr(v *os.FileMode) *OptionalFileMode {
	if v == nil {
		return nil
	}
	return &OptionalFileMode{Value: uint32(*v)}
}

// Copy returns a copy of the OptionalFileMode, or nil if it is nil.
func (o *OptionalFileMode) Copy() *OptionalFileMode {
	if o == nil {
		return nil
	}
	return &OptionalFileMode{Value: o.Value}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalFileMode) Get() *os.FileMode {
	if o == nil {
		return nil
	}
	v := os.FileMode(o.Value)
	return &v
}
