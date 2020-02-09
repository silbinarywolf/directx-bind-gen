package typetrans

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/silbinarywolf/directx-bind-gen/internal/types"
)

func GUIDTypeTranslation() TypeTranslationInfo {
	return TypeTranslationInfo{
		// NOTE(Jae): 2020-01-28
		// Generated in printer.go
		GoType: "GUID",
	}
}

func UIntTypeTranslation() TypeTranslationInfo {
	return TypeTranslationInfo{
		GoType: "uint32",
		Size:   "4",
	}
}

func EnumTypeTranslation() TypeTranslationInfo {
	return TypeTranslationInfo{
		// C-style enum takes 4-bytes (at least in VS2015)
		GoType: "uint32",
		Size:   "4",
	}
}

func GoTypeFromTypeInfo(typeInfo types.TypeInfo) string {
	typeIdent := typeInfo.Ident
	if typeTranslation, ok := builtInTypeTranslation[typeIdent]; ok {
		typeIdent = typeTranslation.GoType
	}
	var b bytes.Buffer
	switch typeInfo := typeInfo.Type.(type) {
	case *types.BasicType:
		b.WriteString(typeIdent)
	case *types.Array:
		for _, dimen := range typeInfo.Dimens {
			b.WriteRune('[')
			b.WriteString(strconv.Itoa(dimen))
			b.WriteRune(']')
		}
		b.WriteString(typeIdent)
	case *types.Union:
		b.WriteString("/*UNION\n{\n")
		//printStructFields(&b, typeInfo.Fields)
		b.WriteString("}*/\n")
	case *types.FunctionPointer:
		b.WriteString("uintptr")
	case *types.Pointer:
		for i := 0; i < typeInfo.Depth; i++ {
			b.WriteRune('*')
		}
		//b.WriteString(GoTypeFromTypeInfo(typeInfo.TypeInfo))
		b.WriteString(typeIdent)
	default:
		panic(fmt.Sprintf("Unhandled struct field type: %T\n", typeInfo))
	}
	ident := b.String()
	if typeTranslation, ok := builtInTypeTranslation[ident]; ok {
		ident = typeTranslation.GoType
	} else {
		ident = strings.Replace(ident, "CONST_VTBL struct ", "", 1)
	}
	return ident
}

type TypeTranslationInfo struct {
	GoType string // ie. uint32
	// Size is the size of the type in bytes
	// retrieved manually by printing sizeof() the type in
	// Visual Studio 2015
	Size string
}

// NOTE(Jae): 2020-01-17
// These sizes were measured using sizeof() with Visual Studio 2015 C++.
// Anything with the size "ptr" is 4 on 32-bit or 8 on 64-bit.
var builtInTypeTranslation = map[string]TypeTranslationInfo{
	// vvvvvvvvvvvvvvvvvvvvvvvvvvv
	// vvvvvvvvvvvvvvvvvvvvvvvvvvv
	"REFGUID": TypeTranslationInfo{
		// TODO: Delete this and handle it properly.
		GoType: "uintptr",
		Size:   "4",
	},
	"REFIID": TypeTranslationInfo{
		// TODO: Delete this and handle it properly.
		GoType: "uintptr",
		Size:   "4",
	},
	// ^^^^^^^^^^^^^^^^^^^^^^^^^^
	// ^^^^^^^^^^^^^^^^^^^^^^^^^^
	"INT": TypeTranslationInfo{
		GoType: "int32",
		Size:   "4",
	},
	"UINT": UIntTypeTranslation(),
	"UINT8": TypeTranslationInfo{
		GoType: "uint8",
		Size:   "1",
	},
	"BYTE": TypeTranslationInfo{
		GoType: "byte",
		Size:   "1",
	},
	"BOOL": TypeTranslationInfo{
		GoType: "uint32",
		// NOTE(Jae): 2020-01-17
		// Windows BOOL type is 4 bytes, when measured in Visual Studio 2015
		// with sizeof(BOOL)
		Size: "4",
	},
	"GUID": GUIDTypeTranslation(),
	"RECT": TypeTranslationInfo{
		GoType: "Rect",
	},
	"FLOAT": TypeTranslationInfo{
		GoType: "float32",
		Size:   "4",
	},
	"float": TypeTranslationInfo{
		GoType: "float32",
		Size:   "4",
	},
	"UINT64": TypeTranslationInfo{
		GoType: "uint64",
		Size:   "8",
	},
	"LPSTR": TypeTranslationInfo{
		GoType: "*byte",
		Size:   "ptr",
	},
	"LPVOID": TypeTranslationInfo{
		// typedef void *LPVOID;
		GoType: "uintptr",
		Size:   "ptr",
	},
	"HANDLE": TypeTranslationInfo{
		// typedef PVOID HANDLE;
		GoType: "uintptr",
		Size:   "ptr",
	},
	"HDC": TypeTranslationInfo{
		// typedef HANDLE HDC;
		GoType: "uintptr",
		Size:   "ptr",
	},
	"HRESULT": TypeTranslationInfo{
		GoType: "uintptr",
		Size:   "ptr",
	},
	"WCHAR": TypeTranslationInfo{
		GoType: "uint16",
		Size:   "2",
	},
	"DWORD": TypeTranslationInfo{
		// A 32-bit unsigned integer. The range is 0 through 4294967295 decimal.
		// https://docs.microsoft.com/en-us/windows/win32/winprog/windows-data-types
		GoType: "uint32",
		Size:   "4",
	},
	"LARGE_INTEGER": TypeTranslationInfo{
		// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-large_integer~r1
		// Represents a 64-bit signed integer value. (union type in C headers)
		GoType: "int64",
		Size:   "8",
	},
	"LUID": TypeTranslationInfo{
		// todo(Jae): 2020-01-28
		// Might need to make this generated in printer.go
		// as its own struct type.

		// Describes a local identifier for an adapter.
		// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-luid
		// typedef struct _LUID {
		//  DWORD LowPart;
		//  LONG  HighPart;
		//} LUID, *PLUID;
		GoType: "int64",
		Size:   "8",
	},
	"IUnknown": TypeTranslationInfo{
		GoType: "uintptr",
		Size:   "ptr",
	},
	"ENUM": EnumTypeTranslation(),
	"LPCSTR": TypeTranslationInfo{
		// An LPCSTR is a 32-bit pointer to a constant null-terminated string of 8-bit Windows (ANSI) characters.
		// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-dtyp/f8d4fe46-6be8-44c9-8823-615a21d17a61
		GoType: "*byte",
		Size:   "ptr",
	},
	"LONG": TypeTranslationInfo{
		GoType: "int32",
		Size:   "4",
	},
	"SIZE_T": TypeTranslationInfo{
		GoType: "uint",
		Size:   "ptr",
	},
	"*char": TypeTranslationInfo{
		GoType: "*byte",
		Size:   "ptr",
	},
	"*const char": TypeTranslationInfo{
		GoType: "*byte",
		Size:   "ptr",
	},
	"*void": TypeTranslationInfo{
		GoType: "uintptr",
		Size:   "ptr",
	},
	"**void": TypeTranslationInfo{
		GoType: "uintptr",
		Size:   "ptr",
	},
	"*const void": TypeTranslationInfo{
		GoType: "uintptr",
		Size:   "ptr",
	},
	"**const void": TypeTranslationInfo{
		GoType: "uintptr",
		Size:   "ptr",
	},
}

func BuiltInTypeTranslation(typeName string) (TypeTranslationInfo, bool) {
	r, ok := builtInTypeTranslation[typeName]
	if !ok {
		return TypeTranslationInfo{}, false
	}
	return r, ok
}

/*
func init() {
	type myStruct struct {
		t bool
	}
	var b bool
	s := myStruct{}
	fmt.Printf("Struct Size of: %v\n", unsafe.Sizeof(s))
	fmt.Printf("Type Size of: %v\n", unsafe.Sizeof(b))
	panic("stop")
}
*/
