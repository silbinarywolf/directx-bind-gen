package printer

import (
	"bytes"
	"strconv"

	"github.com/silbinarywolf/directx-bind-gen/internal/types"
	"github.com/silbinarywolf/directx-bind-gen/internal/typetrans"
)

const interfaceWithGuid = "d3dInterface"

func PrintProject(project *types.Project) []byte {
	enumTypeTranslation := typetrans.EnumTypeTranslation()

	// Output
	var b bytes.Buffer
	b.WriteString("package d3d11\n\n")
	b.WriteString(`
import (
	"syscall"
	"strconv"
	//"reflect"
	"unsafe"
)

type d3dInterface interface {
	guid() guid
}

// Error is returned by all Direct3D11 functions. It encapsulates the error code
// returned by Direct3D. If a function succeeds it will return nil as the Error
// and if it fails you can retrieve the error code using the Code() function.
// You can check the result against the predefined error codes (like
// ERR_DEVICELOST, E_OUTOFMEMORY etc).
type Error interface {
	error
	Code() int32
}

type ErrorValue int32

func (err ErrorValue) Error() string {
	switch err {
	case E_INVALIDARG:
		return "E_INVALIDARG"
	}
	return "unknown error: " + strconv.Itoa(int(err))
}

func (err ErrorValue) Code() int32 {
	return int32(err)
}

func toErr(result uintptr) Error {
	res := ErrorValue(result) // cast to signed int
	if res >= 0 {
		return nil
	}
	return res
}

var (
	d3d11 = syscall.NewLazyDLL("d3d11.dll")
)

`)
	for _, file := range project.Files {
		if len(file.Macros) > 0 {
			for _, record := range file.Macros {
				ident := record.Ident
				value := record.Value.String()
				if ident == value {
					// ignore referencing self duplicates
					continue
				}
				b.WriteString("const ")
				b.WriteString(ident)
				b.WriteString(" = ")
				b.WriteString(value)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		for _, record := range file.Functions {
			ident := record.Ident
			callIdent := "call" + record.Ident
			b.WriteString("var " + callIdent + " = d3d11.NewProc(\"" + record.DLLCall + "\")\n\n")
			b.WriteString("func " + ident)
			printParametersAndReturns(&b, record.Parameters)
			b.WriteString(" {\n")
			b.WriteString("\tret, _, _ := ")
			b.WriteString(callIdent)
			b.WriteString(".Call(\n")
			for _, param := range record.Parameters {
				printArgument(&b, param)
			}
			b.WriteString("\t)\n")
			b.WriteString("\terr = toErr(ret)\n")
			b.WriteString("\treturn\n")
			b.WriteString("}\n\n")

		}
		for _, record := range file.Structs {
			ident := record.Ident
			b.WriteString("type " + ident + " struct {\n")
			printStructFields(&b, record.Fields)
			b.WriteString("}\n\n")
		}
		for _, record := range file.VtblStructs {
			{
				ident := record.Ident
				b.WriteString("type " + ident + " struct {\n")
				printStructFields(&b, record.Fields)
				b.WriteString("}\n\n")
			}

			structIdent := record.NonVtblIdent
			if structIdent == "" {
				panic("Unexpected error. Missing ident information for vtbl struct: " + record.Ident)
			}
			// func (obj *Device) GUID() GUID {
			b.WriteString("func (obj *" + structIdent + ") GUID() ")
			b.WriteString(typetrans.GUIDTypeTranslation().GoType)
			b.WriteString(" {\n")
			// todo(Jae): Get real GUID from header file parsing
			b.WriteString("\t// this is mocked, todo\n")
			b.WriteString("\treturn ")
			b.WriteString(typetrans.GUIDTypeTranslation().GoType)
			b.WriteString("{0x2411e7e1, 0x12ac, 0x4ccf, [8]byte{0xbd, 0x14, 0x97, 0x98, 0xe8, 0x53, 0x4d, 0xc0}}\n")
			b.WriteString("}\n\n")
			for _, field := range record.Fields {
				typeInfo, ok := field.TypeInfo.Type.(*types.FunctionPointer)
				if !ok {
					continue
				}
				methodName := field.Name
				parameters := typeInfo.Parameters
				if parameters[0].Name != "This" {
					panic("Expected first parameter of function pointer to be This.")
				}
				parameterCount := len(parameters)
				parameters = parameters[1:]
				b.WriteString("func (obj *" + structIdent + ") " + methodName)
				printParametersAndReturns(&b, parameters)
				b.WriteString(" {\n")
				// Write init vars
				for _, param := range parameters {
					if param.TypeInfo.GoType == "REFIID" {
						// NOTE(Jae): 2020-01-26
						// After inspecting some of the DirectX11 header files
						// it seems like REFIID is always the 2nd-last parameter.
						// So we assume that here.
						//outParam := parameters[i+1]
						b.WriteString("\tvar refIIDValue uintptr\n") // := unsafe.Pointer(" + outParam.Name + ")\n")
						b.WriteString("\t" + param.Name + "Guid" + " := " + param.Name + ".guid()\n")
						break
					}
				}
				// Write method body
				b.WriteString("\t")
				unusedParameterCount := 0
				switch parameterCount {
				case 0, 1, 2, 3:
					b.WriteString("ret, _, _ := syscall.Syscall(\n")
					unusedParameterCount = 3
				case 4, 5, 6:
					b.WriteString("ret, _, _ := syscall.Syscall6(\n")
					unusedParameterCount = 6
				case 7, 8, 9:
					b.WriteString("ret, _, _ := syscall.Syscall9(\n")
					unusedParameterCount = 9
				case 10, 11, 12:
					b.WriteString("ret, _, _ := syscall.Syscall12(\n")
					unusedParameterCount = 12
				default:
					panic("Unhandled case: Parameter count too big: " + strconv.Itoa(parameterCount))
				}
				b.WriteString("\t\tobj.lpVtbl." + methodName + ",\n")
				b.WriteString("\t\t" + strconv.Itoa(parameterCount) + ",\n")
				b.WriteString("\t\tuintptr(unsafe.Pointer(obj)),\n")
				for i, param := range parameters {
					if param.TypeInfo.GoType == "REFIID" {
						// NOTE(Jae): 2020-01-26
						// After inspecting some of the DirectX11 header files
						// it seems like REFIID is always the 2nd-last parameter.
						// So we assume that here.
						if i != len(parameters)-2 {
							panic("Expected condition. Always expected REFIID to be second last parameter.")
						}
						outParam := parameters[i+1]
						b.WriteString("\t\tuintptr(unsafe.Pointer(&" + outParam.Name + "Guid)),\n")
						b.WriteString("\t\tuintptr(unsafe.Pointer(&refIIDValue)),\n")
						break
					}
					printArgument(&b, param)
				}
				for i := len(parameters); i < unusedParameterCount-1; i++ {
					// Handle unused parameters for Syscall, Syscall6, etc
					b.WriteString("\t\t0,\n")
				}
				b.WriteString("\t)\n")
				b.WriteString("\terr = toErr(ret)\n")
				b.WriteString("\treturn\n")
				//panic(fmt.Sprintf("%v\n", parameters))
				// Release has to be called when finished using the object to free its
				// associated resources.
				/*func (obj *ID3D11Texture2D) Release() uint32 {
					ret, _, _ := syscall.Syscall(
						obj.vtbl.Release,
						1,
						uintptr(unsafe.Pointer(obj)),
						0,
						0,
					)
					return uint32(ret)
				}*/

				b.WriteString("}\n\n")
			}
		}
		if len(file.TypeAliases) > 0 {
			b.WriteString("type (\n")
			for _, typeAlias := range file.TypeAliases {
				alias := typeAlias.Alias
				if builtInTypeTrans, ok := typetrans.BuiltInTypeTranslation(alias); ok {
					alias = builtInTypeTrans.GoType
				}
				ident := typeAlias.Ident
				if ident == alias {
					continue
				}
				b.WriteString("\t" + ident + " " + alias + "\n")
			}
			b.WriteString(")\n\n")
		}
		for _, record := range file.Enums {
			ident := record.Ident
			goType := enumTypeTranslation.GoType
			b.WriteString("type " + ident + " " + goType + "\n")
			b.WriteString("const (\n")
			for _, field := range record.Fields {
				fieldIdent := field.Ident
				value := field.Value.String()
				if fieldIdent == value {
					// ignore referencing self duplicates
					continue
				}
				b.WriteRune('\t')
				b.WriteString(fieldIdent)
				b.WriteRune(' ')
				b.WriteString(ident)
				b.WriteString(" = ")
				b.WriteString(value)
				b.WriteRune('\n')
			}
			b.WriteString(")\n\n")
		}
	}

	r := b.Bytes()
	return r
}

func printArgument(b *bytes.Buffer, param types.StructField) {
	name := param.Name
	if param.IsArrayLen {
		b.WriteString("\t\tuintptr(len(" + name + ")),\n")
		return
	}
	switch param.TypeInfo.Type.(type) {
	case *types.Pointer:
		if param.HasECount {
			//b.WriteString("\t\tuintptr(len(" + name + ")),\n")
			b.WriteString("\t\tuintptr(unsafe.Pointer(&" + name + "[0])),\n")
			break
		}
		if param.IsOut {
			b.WriteString("\t\tuintptr(unsafe.Pointer(&" + name + ")),\n")
			break
		}
		b.WriteString("\t\tuintptr(unsafe.Pointer(" + name + ")),\n")
	case *types.Array:
		b.WriteString("\t\tuintptr(unsafe.Pointer(&" + name + "[0])),\n")
	/*case *types.BasicType:
	if param.Ident != "" && param.Ident[0] == '*' {
		b.WriteString("\t\tuintptr(unsafe.Pointer(" + name + ")),\n")
		break
	}
	b.WriteString("\t\t//" + param.Ident + "\n")
	b.WriteString("\t\tuintptr(" + name + "),\n")*/
	default:
		b.WriteString("\t\tuintptr(" + name + "),\n")
	}
}

func printParametersAndReturns(b *bytes.Buffer, parameters []types.StructField) {
	var outParam *types.StructField
	b.WriteString("(")
	{
		i := 0
		for _, param := range parameters {
			if param.IsArrayLen {
				// Skip as Go users dont need to pass an array len
				// they just pass a slice
				continue
			}
			goType := param.TypeInfo.GoType
			if goType == "" {
				panic("Expecting TypeInfo.GoType string to have value for Name: " + param.Name)
			}
			if goType == "REFIID" {
				if i != 0 {
					b.WriteString(", ")
				}
				// NOTE(Jae): 2020-01-26
				// After inspecting some of the DirectX11 header files
				// it seems like REFIID is always the 2nd-last parameter.
				// So we assume that here.
				outParam = &parameters[i+1]
				b.WriteString(outParam.Name)
				b.WriteRune(' ')
				b.WriteString(interfaceWithGuid)
				break
			}
			if !param.IsOut {
				if i != 0 {
					b.WriteString(", ")
				}
				b.WriteString(param.Name)
				b.WriteRune(' ')
				b.WriteString(goType)
				i++
			}
		}
	}
	b.WriteString(") (")
	{
		i := 0
		for _, param := range parameters {
			if param.IsArrayLen {
				// Skip as Go users dont need to pass an array len
				// they just pass a slice
				continue
			}
			if param.IsOut {
				if i != 0 {
					b.WriteString(", ")
				}
				goType := param.TypeInfo.GoType
				if len(goType) > 0 && goType[0] == '*' {
					goType = goType[1:]
				}
				b.WriteString(param.Name)
				b.WriteRune(' ')
				b.WriteString(goType)
				i++
			}
		}
		if i > 0 {
			b.WriteString(", ")
		}
	}
	if outParam != nil {
		b.WriteString("r ")
		b.WriteString(interfaceWithGuid)
	} else {
		b.WriteString("err Error")
	}
	b.WriteString(")")
}

/*func printParameters(b *bytes.Buffer, parameters []types.StructField, isOut bool) int {
	i := 0
	for _, param := range parameters {
		goType := typetrans.GoTypeFromTypeInfo(param.TypeInfo)
		if goType == "REFIID" {
			if i != 0 {
				b.WriteString(", ")
			}
			// NOTE(Jae): 2020-01-26
			// After inspecting some of the DirectX11 header files
			// it seems like REFIID is always the 2nd-last parameter.
			// So we assume that here.
			outParam = &parameters[i+1]
			b.WriteString(outParam.Name)
			b.WriteRune(' ')
			b.WriteString(interfaceWithGuid)
			break
		}
		if isOut == param.IsOut {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(param.Name)
			b.WriteRune(' ')
			b.WriteString(goType)
			i++
		}
	}
	return i
}*/

func printStructFields(b *bytes.Buffer, fields []types.StructField) {
	if len(fields) == 0 {
		panic("Unexpected error. Found struct with no fields.")
	}
	for _, field := range fields {
		b.WriteRune('\t')
		if field.Name != "" {
			b.WriteString(field.Name)
			b.WriteRune(' ')
		}
		b.WriteString(field.TypeInfo.GoType)
		b.WriteRune('\n')
	}
}
