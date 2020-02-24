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
		record.Parameters = transformParameters(record.Parameters, true)
	}
	for i := 0; i < len(file.Structs); i++ {
		record := &file.Structs[i]
		record.Ident = transformIdent(record.Ident)
		record.Fields = transformParameters(record.Fields, false)
		if record := record.VtblStruct; record != nil {
			record.Ident = transformIdent(record.Ident)
			record.Fields = transformParameters(record.Fields, false)
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

func transformParameters(parameters []types.StructField, isFunction bool) []types.StructField {
	// Annotate with custom metadata
	if isFunction {
		//firstPrevHasECount := false
		for i := 0; i < len(parameters); i++ {
			param := &parameters[i]
			if i+1 < len(parameters) &&
				(strings.HasPrefix(param.Name, "Num") &&
					param.Name != "NumElements") ||
				param.Name == "FeatureLevels" {
				// Hack to handle this case:
				// - OMSetRenderTargets(NumViews)
				// - RSSetViewports(NumViewports)
				nextParam := &parameters[i+1]
				if types.IsECountArray(nextParam) {
					param.Name = nextParam.Name
					nextParam.IsArray = true
					param.IsArrayLen = true
					continue
				}
			}
			if i > 0 {
				// Convert previous param to array length parameter
				prevParam := &parameters[i-1]
				if prevParam.HasECount {
					// Add metadata for this case so we can just pass in a slice for Golang
					if prevParam.TypeInfo.Ident == "D3D_FEATURE_LEVEL" {
						/* lastParam.Name == "NumViews"*/
						param.Name = prevParam.Name
						prevParam.IsArray = true
						param.IsArrayLen = true
						//firstPrevHasECount = false
						continue
					}
				}
			}
		}
	}

	// Transform idents / etc
	for i := 0; i < len(parameters); i++ {
		param := &parameters[i]
		param.Name = transformIdent(param.Name)
		param.TypeInfo.GoType = transformIdent(typetrans.GoTypeFromTypeInfo(param.TypeInfo))
		switch typeInfo := param.TypeInfo.Type.(type) {
		case *types.Pointer:
			if param.HasECount {
				if param.IsArray {
					param.TypeInfo.GoType = "[]" + transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo))
				} else {
					switch typeInfo.Depth {
					case 1:
						param.TypeInfo.GoType = "*" + transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo))
					case 2:
						param.TypeInfo.GoType = "**" + transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo))
						// no-op
						// param.TypeInfo.GoType = transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo.Ident))
					case 3:
						// NOTE(Jae): 2020-02-20
						// Hack that works for the time-being. Need to figure out why this makes things still work
						param.TypeInfo.GoType = "***" + transformIdent(typetrans.GoTypeFromTypeInfo(typeInfo.TypeInfo))
					default:
						panic(fmt.Sprintf("Unhandled pointer depth: %d for %s", typeInfo.Depth, param.Name))
					}
				}
			}
		}

		// docs say its a 32-bit pointer, but sizeof(LPCVOID) is 8-bytes with 64-bit VS2015
		// - https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-dtyp/66996877-9dd4-477d-a811-30e6c1a5525d
		switch param.TypeInfo.Ident {
		case "LPCVOID":
			param.IsDeref = true
		}

		switch typeInfo := param.TypeInfo.Type.(type) {
		case *types.Pointer:
			//if param.TypeInfo.GoType == "uintptr" {
			//	param.IsDeref = true
			//}
			switch typeInfo.Depth {
			case 1:
				switch param.TypeInfo.Ident {
				case "ID3D11Resource": // Resource would ideally convert to a custom interface for Golang, but this is lazier/quicker
					param.IsDeref = true
				case "ID3D10Effect": // Should probably also be a custom interface, but will just make it tagged as interface{} for Go
					param.IsDeref = true
				}
			case 2:
				// Convert C pointer types to our Golang equivalent
				switch param.TypeInfo.Ident {
				case "void",
					"IUnknown":
					param.IsDeref = true
				}
			}
		case *types.FunctionPointer:
			typeInfo.Ident = transformIdent(param.Name)
			typeInfo.Parameters = transformParameters(typeInfo.Parameters, true)
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
	// Remove DX11 namespace
	ident = strings.ReplaceAll(ident, "ID3D11", "")
	ident = strings.ReplaceAll(ident, "D3D11_", "")
	ident = strings.ReplaceAll(ident, "D3D11", "")
	// Remove DX10 namespace
	ident = strings.ReplaceAll(ident, "D3D10_1_", "")
	ident = strings.ReplaceAll(ident, "ID3D10", "")
	ident = strings.ReplaceAll(ident, "D3D10_", "")
	ident = strings.ReplaceAll(ident, "D3D10", "")

	ident = strings.ReplaceAll(ident, "D3D_", "")
	/*for pointerDepth > 0 {
		ident = "*" + ident
		pointerDepth--
	}*/
	return ident
}
