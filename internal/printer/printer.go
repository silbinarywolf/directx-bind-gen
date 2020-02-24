package printer

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/silbinarywolf/directx-bind-gen/internal/types"
	"github.com/silbinarywolf/directx-bind-gen/internal/typetrans"
)

// interfaceWithGuid must be an interface{} as we want the user
// to pass in a pointer-pointer to methods accepting a GUID/output value
const interfaceWithGuid = "interface{}"

func PrintProject(project *types.Project) []byte {
	enumTypeTranslation := typetrans.EnumTypeTranslation()
	constantAlreadyDefinedMap := make(map[string]bool)

	// Output
	var b bytes.Buffer
	b.WriteString("package d3d11\n\n")
	b.WriteString(fmt.Sprintf(`
import (
	"syscall"
	"strconv"
	"reflect"
	"unsafe"
)

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

`))
	for _, file := range project.Files {
		if len(file.Macros) > 0 {
			hasMacro := false
			for _, record := range file.Macros {
				ident := record.Ident
				if _, ok := constantAlreadyDefinedMap[ident]; ok {
					continue
				}
				if strings.HasSuffix(ident, "_H_VERSION__") {
					// ignore version macros, not needed for anything
					continue
				}
				value := record.Value.String()
				if ident == value {
					// ignore referencing self duplicates
					continue
				}
				if !hasMacro {
					b.WriteString("// Macros\n")
					b.WriteString("const (\n")
					hasMacro = true
				}
				b.WriteString("\t")
				b.WriteString(ident)
				b.WriteString(" = ")
				b.WriteString(value)
				b.WriteString("\n")
				constantAlreadyDefinedMap[ident] = true
			}
			if hasMacro {
				b.WriteString(")\n")
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
			printParameterInitVars(&b, record.Parameters)
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
			structIdent := record.Ident

			b.WriteString("type " + structIdent + " struct {\n")
			printStructFields(&b, record.Fields)
			b.WriteString("}\n\n")

			// Add GUID
			// ie. "839d1216-bb2e-412b-b7f4-a9dbebe08ed1"
			if len(record.GUID) > 0 {
				// func (obj *Device) GUID() GUID {
				b.WriteString("// GUID returns a string representing a Class identifier (ID) for COM objects\n")
				b.WriteString("// ")
				b.WriteString(record.GUID)
				b.WriteString("\n")
				b.WriteString("func (obj *" + structIdent + ") GUID() ")
				b.WriteString(typetrans.GUIDTypeTranslation().GoType)
				b.WriteString(" {\n")
				b.WriteString("\treturn ")
				b.WriteString(typetrans.GUIDTypeTranslation().GoType)
				// Print "Data1" field
				b.WriteString("{0x")
				b.WriteString(record.GUID[:8])
				// Print "Data2" field
				b.WriteString(", 0x")
				b.WriteString(record.GUID[9:13])
				// Print "Data3" field
				b.WriteString(", 0x")
				b.WriteString(record.GUID[14:18])
				// Print "Data4" field
				b.WriteString(", [8]byte{")
				b.WriteString("0x")
				b.WriteString(record.GUID[19:21])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[21:23])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[24:26])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[26:28])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[28:30])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[30:32])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[32:34])
				b.WriteString(", 0x")
				b.WriteString(record.GUID[34:36])
				b.WriteString("}}\n")
				b.WriteString("}\n\n")
			}

			// Generate Vtbl
			if record := record.VtblStruct; record != nil {
				// Add vtbl struct and fields
				b.WriteString("type " + record.Ident + " struct {\n")
				printStructFields(&b, record.Fields)
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
					printParameterInitVars(&b, parameters)
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
					for _, param := range parameters {
						printArgument(&b, param)
					}
					for i := len(parameters); i < unusedParameterCount-1; i++ {
						// Handle unused parameters for Syscall, Syscall6, etc
						b.WriteString("\t\t0,\n")
					}
					b.WriteString("\t)\n")
					b.WriteString("\terr = toErr(ret)\n")
					b.WriteString("\treturn\n")
					b.WriteString("}\n\n")
				}
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
	if param.IsArray {
		//b.WriteString("\t\tuintptr(len(" + name + ")),\n")
		b.WriteString("\t\tuintptr(unsafe.Pointer(&" + name + "[0])),\n")
		return
	}
	if param.IsArrayLen {
		b.WriteString("\t\tuintptr(len(" + name + ")),\n")
		return
	}
	if param.IsDeref {
		b.WriteString("\t\tuintptr(unsafe.Pointer(" + name)
		b.WriteString("Pointer")
		b.WriteString(")),\n")
		return
	}
	switch param.TypeInfo.Type.(type) {
	case *types.Pointer:
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
		if param.TypeInfo.GoType == typetrans.GUIDTypeTranslation().GoType {
			// NOTE(Jae): 2020-02-09
			// A bit of a hack to make GUID structs work. Might add
			// "IsStruct" boolean in the future
			b.WriteString("\t\tuintptr(unsafe.Pointer(&" + name + ")),\n")
		} else {
			b.WriteString("\t\tuintptr(" + name + "),\n")
		}
	}
}

func printParametersAndReturns(b *bytes.Buffer, parameters []types.StructField) {
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
			isDeref := param.IsDeref
			if param.IsOut &&
				!isDeref {
				continue
			}
			if i != 0 {
				b.WriteString(", ")
			}

			if param.IsDeref {
				b.WriteString(param.Name)
				b.WriteRune(' ')
				b.WriteString(interfaceWithGuid)
			} else {
				b.WriteString(param.Name)
				b.WriteRune(' ')
				b.WriteString(goType)
			}
			i++
		}
	}
	b.WriteString(") (")
	{
		i := 0
		for _, param := range parameters {
			if !param.IsOut {
				continue
			}
			if param.IsDeref {
				continue
				/*if ind > 0 {
					// Skip as we put deref's as input parameters
					// - REFIID riid
					// - __RPC__deref_out  void **ppvObject
					prevParam := parameters[ind-1]
					if prevParam.TypeInfo.Ident == "REFIID" {
						continue
					}
				}*/
			}
			if param.IsArrayLen {
				// Skip as Go users dont need to pass an array len
				// they just pass a slice
				continue
			}
			if i != 0 {
				b.WriteString(", ")
			}
			goType := param.TypeInfo.GoType
			if len(goType) > 0 && goType[0] == '*' {
				// Remove parameter so that
				// - **d3d11.Device becomes *d3d11.Device
				goType = goType[1:]
			}
			b.WriteString(param.Name)
			b.WriteRune(' ')
			b.WriteString(goType)
			i++
		}
		if i > 0 {
			b.WriteString(", ")
		}
	}
	b.WriteString("err Error")
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

func printParameterInitVars(b *bytes.Buffer, parameters []types.StructField) {
	// Write init vars
	for _, param := range parameters {
		if param.IsDeref {
			refName := param.Name + "Ref"
			pointerName := param.Name + "Pointer"

			b.WriteString("\t")
			b.WriteString(refName)
			b.WriteString(" := ")
			b.WriteString("reflect.ValueOf(")
			b.WriteString(param.Name)
			b.WriteString(")\n")
			b.WriteString("\tif " + refName + ".Kind() != reflect.Ptr {\n")
			b.WriteString("\t\tpanic(\"Expected a pointer\")\n")
			b.WriteString("\t}\n")
			b.WriteString("\t")
			b.WriteString(pointerName)
			b.WriteString(" := ")
			b.WriteString(refName)
			b.WriteString(".Pointer()\n")
			b.WriteString("\n")
		}
	}
}

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
		if field.IsDeref {
			b.WriteString("uintptr")
		} else {
			b.WriteString(field.TypeInfo.GoType)
		}

		b.WriteRune('\n')
	}
}
