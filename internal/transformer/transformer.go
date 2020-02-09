package transformer

import (
	"fmt"
	"strings"

	"github.com/silbinarywolf/directx-bind-gen/internal/types"
	"github.com/silbinarywolf/directx-bind-gen/internal/typetrans"
)

// Transform applies additional custom rules and transformations
// to make printing out to modern languages easier
func Transform(file *types.File) {
	usedConstants := make(map[string]bool)

	for i := 0; i < len(file.Functions); i++ {
		record := &file.Functions[i]
		record.Ident = transformIdent(record.Ident)
		record.Parameters = transformParameters(record.Parameters)
	}
	for i := 0; i < len(file.Structs); i++ {
		record := &file.Structs[i]
		record.Ident = transformIdent(record.Ident)
		record.Fields = transformParameters(record.Fields)
		if record := record.VtblStruct; record != nil {
			record.Ident = transformIdent(record.Ident)
			record.Fields = transformParameters(record.Fields)
		}
	}
	for i := 0; i < len(file.TypeAliases); i++ {
		record := &file.TypeAliases[i]
		record.Ident = transformIdent(record.Ident)
		record.Alias = transformIdent(record.Alias)
	}
	for i := 0; i < len(file.Enums); i++ {
		record := &file.Enums[i]
		record.Ident = transformIdent(record.Ident)
		for i := 0; i < len(record.Fields); i++ {
			field := &record.Fields[i]
			field.Ident = transformIdent(field.Ident)
			// NOTE(Jae): 2020-02-02
			// Do this so we can transform the constants
			// D3D11_COLOR_WRITE_ENABLE_RED
			field.RawValue = transformIdent(field.RawValue)
			if _, ok := usedConstants[field.Ident]; ok {
				// Remove duplicates
				record.Fields = append(record.Fields[:i], record.Fields[i+1:]...)
				i--
				continue
			}
			usedConstants[field.Ident] = true
		}
	}
}

func transformParameters(parameters []types.StructField) []types.StructField {
	firstPrevHasECount := false
	for i := 0; i < len(parameters); i++ {
		param := &parameters[i]
		if firstPrevHasECount {
			// Delete this field if previous has an _ecount
			// annotation, as its most likely a dynamic array
			//parameters = append(parameters[:i], parameters[i+1:]...)
			/*if i >= len(parameters) {
				break
			}
			param = &parameters[i]*/

			// Convert previous param to array length parameter
			lastParam := &parameters[i-1]
			param.Name = lastParam.Name
			param.IsArrayLen = true
			firstPrevHasECount = false
			continue
		}
		param.Name = transformIdent(param.Name)
		param.TypeInfo.GoType = transformIdent(typetrans.GoTypeFromTypeInfo(param.TypeInfo))
		if param.HasECount {
			switch typeInfo := param.TypeInfo.Type.(type) {
			case *types.Pointer:
				switch typeInfo.Depth {
				case 1:
					param.TypeInfo.GoType = "[]" + transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo))
				case 2:
					param.TypeInfo.GoType = "[]" + transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo))
					// no-op
					// param.TypeInfo.GoType = transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo.Ident))
				default:
					panic(fmt.Sprintf("Unhandled pointer depth: %d", typeInfo.Depth))
				}
			default:
				panic(fmt.Sprintf("Unhandled ecount type: %T", param.TypeInfo))
			}
		} else {
			param.TypeInfo.GoType = transformIdent(typetrans.GoTypeFromTypeInfo(param.TypeInfo))
		}
		firstPrevHasECount = param.HasECount
		switch typeInfo := param.TypeInfo.Type.(type) {
		case *types.Pointer:
			param.IsDeref = param.TypeInfo.Ident == "void" && typeInfo.Depth == 2
		case *types.FunctionPointer:
			typeInfo.Ident = transformIdent(param.Name)
			typeInfo.Parameters = transformParameters(typeInfo.Parameters)
		}
	}
	return parameters
}

func transformIdent(ident string) string {
	/*pointerDepth := 0
	for len(ident) > 0 && ident[0] == '*' {
		ident = ident[1:]
		pointerDepth++
	}*/
	ident = strings.ReplaceAll(ident, "ID3D11", "")
	ident = strings.ReplaceAll(ident, "D3D11_", "")
	ident = strings.ReplaceAll(ident, "D3D11", "")
	ident = strings.ReplaceAll(ident, "D3D_", "")
	/*for pointerDepth > 0 {
		ident = "*" + ident
		pointerDepth--
	}*/
	return ident
}
