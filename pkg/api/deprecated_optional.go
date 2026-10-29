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

import "os"

// String creates an Optional wrapper from its argument.
//
// Deprecated: Use NewString, StringPtr, or (*OptionalString).Copy() instead.
func String(v any) *OptionalString {
	switch o := v.(type) {
	case string:
		return NewString(o)
	case *string:
		return StringPtr(o)
	case *OptionalString:
		return o.Copy()
	default:
		return nil
	}
}

// RepeatedString creates an Optional wrapper from its argument.
//
// Deprecated: Use NewRepeatedString, RepeatedStringPtr, or (*OptionalRepeatedString).Copy() instead.
func RepeatedString(v any) *OptionalRepeatedString {
	switch o := v.(type) {
	case []string:
		return NewRepeatedString(o)
	case *[]string:
		return RepeatedStringPtr(o)
	case *OptionalRepeatedString:
		return o.Copy()
	default:
		return nil
	}
}

// Int creates an Optional wrapper from its argument.
//
// Deprecated: Use NewInt, IntPtr, or (*OptionalInt).Copy() instead.
func Int(v any) *OptionalInt {
	switch o := v.(type) {
	case int:
		return NewInt(o)
	case *int:
		return IntPtr(o)
	case *OptionalInt:
		return o.Copy()
	default:
		return nil
	}
}

// Int32 creates an Optional wrapper from its argument.
//
// Deprecated: Use NewInt32, Int32Ptr, or (*OptionalInt32).Copy() instead.
func Int32(v any) *OptionalInt32 {
	switch o := v.(type) {
	case int32:
		return NewInt32(o)
	case *int32:
		return Int32Ptr(o)
	case *OptionalInt32:
		return o.Copy()
	default:
		return nil
	}
}

// UInt32 creates an Optional wrapper from its argument.
//
// Deprecated: Use NewUInt32, UInt32Ptr, or (*OptionalUInt32).Copy() instead.
func UInt32(v any) *OptionalUInt32 {
	switch o := v.(type) {
	case uint32:
		return NewUInt32(o)
	case *uint32:
		return UInt32Ptr(o)
	case *OptionalUInt32:
		return o.Copy()
	default:
		return nil
	}
}

// Int64 creates an Optional wrapper from its argument.
//
// Deprecated: Use NewInt64, Int64Ptr, or (*OptionalInt64).Copy() instead.
func Int64(v any) *OptionalInt64 {
	switch o := v.(type) {
	case int:
		return NewInt64(int64(o))
	case uint:
		return NewInt64(int64(o))
	case uint64:
		return NewInt64(int64(o))
	case int64:
		return NewInt64(o)
	case *int64:
		return Int64Ptr(o)
	case *uint64:
		if o == nil {
			return nil
		}
		return NewInt64(int64(*o))
	case *OptionalInt64:
		return o.Copy()
	default:
		return nil
	}
}

// UInt64 creates an Optional wrapper from its argument.
//
// Deprecated: Use NewUInt64, UInt64Ptr, or (*OptionalUInt64).Copy() instead.
func UInt64(v any) *OptionalUInt64 {
	switch o := v.(type) {
	case int:
		return NewUInt64(uint64(o))
	case uint:
		return NewUInt64(uint64(o))
	case int64:
		return NewUInt64(uint64(o))
	case uint64:
		return NewUInt64(o)
	case *int64:
		if o == nil {
			return nil
		}
		return NewUInt64(uint64(*o))
	case *uint64:
		return UInt64Ptr(o)
	case *OptionalUInt64:
		return o.Copy()
	default:
		return nil
	}
}

// Bool creates an Optional wrapper from its argument.
//
// Deprecated: Use NewBool, BoolPtr, or (*OptionalBool).Copy() instead.
func Bool(v any) *OptionalBool {
	switch o := v.(type) {
	case bool:
		return NewBool(o)
	case *bool:
		return BoolPtr(o)
	case *OptionalBool:
		return o.Copy()
	default:
		return nil
	}
}

// FileMode creates an Optional wrapper from its argument.
//
// Deprecated: Use NewFileMode, FileModePtr, or (*OptionalFileMode).Copy() instead.
func FileMode(v any) *OptionalFileMode {
	switch o := v.(type) {
	case os.FileMode:
		return NewFileMode(o)
	case *os.FileMode:
		return FileModePtr(o)
	case *OptionalFileMode:
		return o.Copy()
	case uint32:
		return NewFileMode(os.FileMode(o))
	default:
		return nil
	}
}
