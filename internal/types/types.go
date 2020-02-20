package types

import (
	"strconv"
)

type Project struct {
	Files []File
}

type Value struct {
	// UInt32Value is set if the value is an uint32 type
	UInt32Value *uint32
	// StringValue is set if the value is a string type
	StringValue *string
	// RawValue is the value as it appears in C-code, this is
	// used in code generation if we can't compute the value
	RawValue string
}

// String will get the value that
func (v *Value) String() string {
	if v.UInt32Value != nil {
		return strconv.FormatUint(uint64(*v.UInt32Value), 10)
	}
	if v.StringValue != nil {
		return *v.StringValue
	}
	return v.RawValue
}

type File struct {
	Filename    string
	Structs     []Struct
	Functions   []Function
	TypeAliases []TypeAlias
	Enums       []Enum
	Macros      []Macro
}

type Macro struct {
	Ident string
	Value
}

type Function struct {
	Ident      string
	DLLCall    string
	Parameters []StructField
}

type TypeAlias struct {
	Ident string
	Alias string
}

type Struct struct {
	Ident  string
	Fields []StructField

	// Vtbl for the struct
	VtblStruct *Struct

	// GUID string for the struct (applies only COM interface types)
	GUID string
}

type StructField struct {
	Name string

	// IsOut is true when the field has an __out annotation
	IsOut bool
	// HasECount is true when the field has an __in_ecount_opt
	// field, this normally means the next field is a UINT
	// representing how many are in an array
	HasECount bool
	// IsArray is when the field is most likely a dynamic array
	IsArray bool
	// IsArrayLen is true when field(s) are meant to represent
	// the length of an array of data.
	IsArrayLen bool
	// IsDeref is true when a field has a __deref annotation
	IsDeref bool

	TypeInfo TypeInfo
}

type Enum struct {
	Ident  string
	Fields []EnumField
}

type EnumField struct {
	Ident string
	Value
}

type TypeInfo struct {
	// Name is the name of the type.  "Basic", "Pointer"
	Name string
	// Type is the type information
	Type Type
	// Ident is the type parsed from .h file
	Ident string
	// GoType is generated based on type
	GoType string
}

type Type interface {
	isType()
}

type BasicType struct {
}

func (*BasicType) isType() {}

func NewBasicType(ident string, data BasicType) TypeInfo {
	return TypeInfo{
		Name:  "Basic",
		Ident: ident,
		Type:  &data,
	}
}

type Array struct {
	Dimens []int // Ellipsis nodes for [...]T array types, nil for slice types
}

func (*Array) isType() {}

func NewArray(ident string, data Array) TypeInfo {
	return TypeInfo{
		Name:  "Array",
		Ident: ident,
		Type:  &data,
	}
}

type Union struct {
	Fields []StructField
}

func (*Union) isType() {}

func NewUnion(data Union) TypeInfo {
	return TypeInfo{
		Name: "Union",
		Type: &data,
	}
}

type FunctionPointer struct {
	Ident      string
	Parameters []StructField
}

func (*FunctionPointer) isType() {}

func NewFunctionPointer(data FunctionPointer) TypeInfo {
	return TypeInfo{
		Name: "FunctionPointer",
		Type: &data,
	}
}

type Pointer struct {
	Depth    int
	TypeInfo TypeInfo
}

func (*Pointer) isType() {}

func NewPointer(ident string, data Pointer) TypeInfo {
	return TypeInfo{
		Name:  "Pointer",
		Ident: ident,
		Type:  &data,
	}
}

func IsECountArray(param *StructField) bool {
	//fmt.Printf("param: %s\n", param.Name)
	if param.Name == "ppRenderTargetViews" {
		return false
	}
	typeInfo, ok := param.TypeInfo.Type.(*Pointer)
	if !ok {
		return false
	}
	switch typeInfo.Depth {
	case 1:
		return true
	case 2:
		return true
	}
	return false
}
