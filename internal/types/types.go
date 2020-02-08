package types

type Project struct {
	Files []File
}

type Value struct {
	// RawValue is the value as it appears in C-code
	RawValue string
	// UInt32Value is set if the value is an uint32 type
	UInt32Value *uint32
	// StringValue is set if the value is a string type
	StringValue *string
}

type File struct {
	Filename    string
	Structs     []Struct
	Functions   []Function
	TypeAliases []TypeAlias
	VtblStructs []Struct
	Enums       []Enum
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

	// For Vtbl structs only
	NonVtblIdent string
}

type StructField struct {
	TypeInfo TypeInfo
	Name     string

	// IsOut is true when the field has an __out
	// annotation before it
	IsOut bool
	// HasECount is true when the field has an __in_ecount_opt
	// field, this normally means the next field is a UINT
	// representing how many are in an array
	HasECount  bool
	IsArrayLen bool
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
	Name  string
	Type  Type
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

func NewBasicType(data BasicType) TypeInfo {
	return TypeInfo{
		Name: "Basic",
		Type: &data,
	}
}

type Array struct {
	Dimens []int // Ellipsis nodes for [...]T array types, nil for slice types
}

func (*Array) isType() {}

func NewArray(data Array) TypeInfo {
	return TypeInfo{
		Name: "Array",
		Type: &data,
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
	TypeInfo TypeInfo
	Depth    int
}

func (*Pointer) isType() {}

func NewPointer(data Pointer) TypeInfo {
	return TypeInfo{
		Name: "Pointer",
		Type: &data,
	}
}
