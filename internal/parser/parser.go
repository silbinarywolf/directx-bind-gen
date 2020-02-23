package parser

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
	"unicode/utf8"

	"github.com/silbinarywolf/directx-bind-gen/internal/types"
	"github.com/silbinarywolf/directx-bind-gen/internal/typetrans"
)

var precedence = map[string]int{
	"(":  1,
	")":  1,
	"||": 2,
	"&&": 2,
	"==": 3,
	"!=": 3, // NOTE(Jae): 2018(?) - Didn't check this against other langs
	// NOTE(Jae): 2020-02-18
	// Need << and >> to have a precdence lower than +, -, /, *
	// otherwise DXGI_USAGE_SHADER_INPUT ends up not being expected 16
	// (im unsure of if this means the parentheses are handled incorrectly
	// but whatever for this use-case for now)
	"|":  4,
	"<<": 4,
	">>": 4,
	"+":  5,
	"-":  5,
	"/":  5,
	"*":  5,
	// NOTE(Jae): 2020-02-18
	// Everything under here is naively copied from and untested
	// https://en.wikipedia.org/wiki/Operators_in_C_and_C%2B%2B
	"<":  9,
	"<=": 9,
	">":  9,
	">=": 9,
}

func OperatorPrecedence(operator string) int {
	r, ok := precedence[operator]
	if !ok {
		panic("Invalid operator, no precedence found: " + operator)
	}
	return r
}

func IsOperator(operator string) bool {
	_, ok := precedence[operator]
	return ok
	/*return c == '<' ||
	c == '+' ||
	c == '-' ||
	operator == "<<" ||
	operator == ">>" ||
	operator == "++" ||
	operator == "|"*/
}

func IsNumber(str string) bool {
	c := str[0]
	return c >= '0' && c <= '9'
}

func IsIdent(str string) bool {
	c := str[0]
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func NumberToFloat64(str string) float64 {
	if len(str) > 1 &&
		str[0] == '0' &&
		str[1] == 'x' {
		i, err := strconv.ParseInt(str[2:], 16, 0)
		if err != nil {
			panic("Failed to parse hex value.")
		}
		return float64(i)
	}
	i, err := strconv.ParseInt(str, 10, 0)
	if err != nil {
		panic(fmt.Sprintf("Tried to parse int and failed: %s", str))
	}
	return float64(i)
}

func ParseFile(filename string) types.File {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// structIdentToGuid is used to attach GUID/IID data
	// to a struct
	structIdentToGuid := make(map[string]string)
	vtblStructIdentToData := make(map[string]*types.Struct)

	//
	defineValuesMap := make(map[string]string)

	var file types.File
	file.Filename = filename

	var s scanner.Scanner
	s.Init(f)
	s.Filename = filename
	s.Mode = scanner.GoTokens //^= scanner.SkipComments // don't skip comments
MainLoop:
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "#":
			s.Scan()
			switch s.TokenText() {
			case "define", "DEFINE":
				//constIdentPos := s.Pos()
				s.Scan()
				constIdent := s.TokenText()
				if constIdent == "INTERFACE" {
					// Ignore cases like:
					// #undef INTERFACE
					// #define INTERFACE ID3D10ShaderReflection1
					continue
				}
				if constIdent == "IID_ID3DBlob" {
					// Ignore cases like:
					// - #define IID_ID3DBlob IID_ID3D10Blob
					continue
				}
				if constIdent == "D3DCOMPILER_DLL_W" ||
					constIdent == "D3DCOMPILER_DLL_A" ||
					constIdent == "D3DCOMPILER_DLL" {
					// Ignore cases like:
					// - #define D3DCOMPILER_DLL_W L"d3dcompiler_43.dll"
					// - #define D3DCOMPILER_DLL_A "d3dcompiler_43.dll"
					//
					// Might just need a rule to ignore certain strings?
					continue
				}
				// A few cases to handle:
				// - #define _INCLUDE_H
				// - #define D3D_CONST ( 3 )
				// - #DEFINE DX_VERSION 455
				var exprTokens []string
				{
					skipThisMacro := false
					oldMode := s.Mode
					oldWhitespace := s.Whitespace
					// https://golang.org/pkg/text/scanner/#example__whitespace
					//s.Mode ^= scanner.GoWhitespace
					//s.Whitespace ^= 1<<'\t' | 1<<'\n' // don't skip tabs and new lines
					// TODO(Jae): confirm this works
					s.Whitespace ^= 1 << '\n'
					for {
						prevPos := s.Pos()
						s.Scan()
						nextPos := s.Pos()
						v := s.TokenText()
						v = strings.TrimSpace(v)
						if v == "\\" {
							continue
						}
						if v == "" || v == "\n" {
							break
						}
						// Handle cases like:
						// - #define MAKE_D3D11_HRESULT( code )  MAKE_HRESULT( 1, _FACD3D11, code )
						if v == "(" {
							// Ignore non-trivial macros like:
							// - #define __in_range(x, y)
							if len(exprTokens) == 0 &&
								prevPos.Offset == nextPos.Offset-1 {
								skipThisMacro = true
								break
							}
							// Ignore non-trivial macros like:
							// - #define MAKE_D3D11_HRESULT( code )  MAKE_HRESULT( 1, _FACD3D11, code )
							if len(exprTokens) > 0 &&
								IsIdent(exprTokens[len(exprTokens)-1]) &&
								// ie. 49 == 50-1, for the case MAKE_HRESULT
								prevPos.Column == nextPos.Column-1 {
								skipThisMacro = true
								break
							}
						}
						exprTokens = append(exprTokens, v)
					}
					s.Mode = oldMode
					s.Whitespace = oldWhitespace
					if skipThisMacro {
						continue MainLoop
					}
				}
				if len(exprTokens) == 0 {
					continue
				}

				// Based on a Pratt Expression Parser
				// - http://journal.stuffwithstuff.com/2011/03/19/pratt-parsers-expression-parsing-made-easy/
				infixNodes := make([]string, 0, 10)
				{
					//hasNumberOrOperator := false
					parenOpenCount := 0
					parenCloseCount := 0
					//expectOperator := false
					operatorNodes := make([]string, 0, 10)
				Loop:
					for len(exprTokens) > 0 {
						t := exprTokens[0]
						exprTokens = exprTokens[1:]
						switch t {
						//case ",":
						// Ignore function macros like:
						// - #define __in_range(x, y)
						//continue MainLoop
						case "(":
							/*if len(infixNodes) > 0 &&
								IsIdent(infixNodes[len(infixNodes)-1]) {
								// Ignore "complex" macros like:
								// - #define MAKE_D3D11_HRESULT( code )  MAKE_HRESULT( 1, _FACD3D11, code )
								continue MainLoop
							}*/
							parenOpenCount++
						case ")":
							// If hit end
							if parenCloseCount == 0 && parenOpenCount == 0 {
								break Loop
							}
							parenCloseCount++
							// I actually dont remember why this needs to be here.
							// But OK. Copy-pasted from old expression evaluation code
							// on an old compiler project
							if len(operatorNodes) > 0 {
								topOperatorNode := operatorNodes[len(operatorNodes)-1]
								if topOperatorNode == "(" {
									infixNodes = append(infixNodes, topOperatorNode)
									operatorNodes = operatorNodes[:len(operatorNodes)-1]
								}
							}
						case "L":
							// ( 1L << (0 + 4) )
							// Ignore for now, its to hint that this macro is:
							// "....an integer constant which has long int type instead of int."
							continue
						case "UL":
							// ( 1UL )
							// Ignore for now, its to hint that this macro is:
							// "....an integer constant which has unsigned long int type instead of int."
							continue
						case "f":
							// add f to number, ie. "1.0f"
							if len(infixNodes) == 0 {
								panic("Unexpected error. Cannot add f to number for #define: " + constIdent)
							}
							// Ignore for now. Its a hint that this type is a float.
							//infixNodes[len(infixNodes)-1] += "f"
						default:
							if IsNumber(t) || IsIdent(t) {
								infixNodes = append(infixNodes, t)
								continue
							}
							if IsOperator(t) {
								// Handle <<, ++, etc
								if len(operatorNodes) > 0 {
									prevOp := operatorNodes[len(operatorNodes)-1]
									if t == prevOp {
										operatorNodes = operatorNodes[:len(operatorNodes)-1]
										t += prevOp
									}
								}
								// Handle operators
								for len(operatorNodes) > 0 {
									topOperatorNode := operatorNodes[len(operatorNodes)-1]
									if OperatorPrecedence(topOperatorNode) < OperatorPrecedence(t) {
										break
									}
									operatorNodes = operatorNodes[:len(operatorNodes)-1]
									infixNodes = append(infixNodes, topOperatorNode)
								}
								operatorNodes = append(operatorNodes, t)
								continue
							}
							panic("Unhandled expression token: " + t + " for #define: " + constIdent)
						}
					}

					for len(operatorNodes) > 0 {
						topOperatorNode := operatorNodes[len(operatorNodes)-1]
						operatorNodes = operatorNodes[:len(operatorNodes)-1]
						infixNodes = append(infixNodes, topOperatorNode)
					}
					if parenOpenCount != parenCloseCount {
						// todo(Jae): better error message
						panic("Mismatching paren open and close count")
					}
				}
				// Evaluate expression
				stack := make([]string, 0, 10)
				for len(infixNodes) > 0 {
					t := infixNodes[0]
					infixNodes = infixNodes[1:]
					if IsNumber(t) {
						stack = append(stack, t)
						continue
					}
					if IsOperator(t) {
						rightValue := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						if len(stack) == 0 {
							//if t == "<<" {
							//	panic("Cant prefix a value with: " + t)
							//}
							// Operator only, ie. -42
							// ie. t = "-", rightValue="42"
							stack = append(stack, t+rightValue)
							continue
						}
						leftValue := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						leftValueFloat64 := NumberToFloat64(leftValue)
						rightValueFloat64 := NumberToFloat64(rightValue)
						switch t {
						case "+":
							result := leftValueFloat64 + rightValueFloat64
							//fmt.Printf("Result: %v\n", result)
							stack = append(stack, fmt.Sprintf("%v", result))
							//panic("TODO: handle operator'ing two values together:\n" + leftValue + " " + t + " " + rightValue + " for #define: " + constIdent)
							continue
						case "<<":
							result := uint64(leftValueFloat64) << uint64(rightValueFloat64)
							//fmt.Printf("result: %d for const: %s\n", result, constIdent)
							stack = append(stack, fmt.Sprintf("%d", result))
						case "|":
							result := uint64(leftValueFloat64) | uint64(rightValueFloat64)
							stack = append(stack, fmt.Sprintf("%d", result))
						default:
							panic("TODO: handle operator'ing two values together:\n" + leftValue + " " + t + " " + rightValue + " for #define: " + constIdent)
						}
						continue
					}
					if IsIdent(t) {
						v, ok := defineValuesMap[t]
						if !ok {
							panic("Unable to find existing identifier: " + t + " for this #define: " + constIdent)
						}
						stack = append(stack, v)
						continue
					}
					panic("Unhandled evaluation token: " + t + " for #define: " + constIdent)
				}
				if len(stack) == 0 {
					panic("Unexpected error, empty stack from evaluating expression for #define: " + constIdent)
				}
				if len(stack) > 1 {
					panic(fmt.Sprintf("Unexpected error, stack size is %d instead of 1 (%v) for #define: %s", len(stack), stack, constIdent))
				}
				result := stack[0]

				defineValuesMap[constIdent] = result
				// defineValuesMap
				/*fmt.Printf(`
					#define: %s
					infix Nodes: %v
				`, constIdent, infixNodes)
				if len(infixNodes) > 1 {
					panic(fmt.Sprintf("infix: %v\n", infixNodes))
				}*/

				// Add parsed macro
				record := types.Macro{
					Ident: constIdent,
				}
				record.StringValue = new(string)
				*record.StringValue = result
				file.Macros = append(file.Macros, record)
			}
		case "MIDL_INTERFACE":
			s.Scan()
			if tok := s.TokenText(); tok != "(" {
				panic(s.String() + ": unexpected token: " + tok + " after MIDL_INTERFACE macro")
			}
			s.Scan()
			guid := s.TokenText()
			if guid[0] != '"' {
				panic(s.String() + ": unexpected guid value doesn't start with \": " + guid)
			}
			guid = guid[1 : len(guid)-1] // trim quotes off either side
			s.Scan()
			if tok := s.TokenText(); tok != ")" {
				panic(s.String() + ": unexpected token: " + tok + " after MIDL_INTERFACE data: " + guid)
			}
			s.Scan()
			structName := s.TokenText()
			structIdentToGuid[structName] = guid
		case "typedef":
			s.Scan()
			kind := s.TokenText()
			s.Scan()
			name := s.TokenText()
			switch kind {
			case "UINT":
				file.TypeAliases = append(file.TypeAliases, types.TypeAlias{
					Ident: name,
					Alias: typetrans.UIntTypeTranslation().GoType,
				})
			case "enum":
				s.Scan()
				if t := s.TokenText(); t != "{" {
					continue
				}
				lastValue := uint32(0)
				data := types.Enum{}
				for {
					s.Scan()
					kind := s.TokenText()
					if kind == "}" {
						// This case occurs if enum has "," on last item
						break
					}
					s.Scan() // =
					tok := s.TokenText()
					if tok == "=" {
						s.Scan()
						rawValue, isEndOfEnum := parseEnumExpr(&s, name)
						enumField := types.EnumField{
							Ident: kind,
						}
						enumField.RawValue = rawValue
						if evalValue := tryEvaluateExpr(rawValue); evalValue != nil {
							switch value := evalValue.(type) {
							case string:
								enumField.StringValue = &value
							case uint32:
								enumField.UInt32Value = &value
								lastValue = *enumField.UInt32Value
							default:
								panic(fmt.Sprintf("Unhandled evaluated expression type: %T", value))
							}
						}
						data.Fields = append(data.Fields, enumField)
						if isEndOfEnum {
							break
						}
						continue
					} else if tok == "," {
						lastValue++
						enumField := types.EnumField{
							Ident: kind,
						}
						enumField.UInt32Value = &lastValue
						continue
					}
					panic(s.String() + ": unexpected token: = or , but got " + tok + " after enum field value: " + kind)
				}
				s.Scan()
				data.Ident = s.TokenText()
				file.Enums = append(file.Enums, data)
			case "struct":
				s.Scan()
				if t := s.TokenText(); t != "{" {
					// For debugging
					// fmt.Printf("skip struct: %s\nkind: %s\n", name, kind)
					continue
				}

				// Get struct macro header
				//fmt.Printf("struct: %s\nkind: %s\n", name, kind)

				// Get struct fields
				data := types.Struct{}
				data.Fields = parseStructFields(&s)
				s.Scan()
				// Scan and get proper struct name. (ie. "typedef struct _MyStruct {} MyStruct;")
				// I say proper because the last one is what we generally want.
				data.Ident = s.TokenText()
				s.Scan() // ;
				if tok := s.TokenText(); tok != ";" {
					panic(s.String() + ": unexpected token: " + tok + " at end of struct: " + data.Ident + "expected ;")
				}

				isVtbl := len(data.Fields) > 0 && data.Fields[0].Name == "BEGIN_INTERFACE"
				if isVtbl {
					data.Fields = data.Fields[1:]
				}
				if isVtbl {
					vtblStructIdentToData[data.Ident] = &data
				} else {
					file.Structs = append(file.Structs, data)
				}
			case "interface":
				// ignore, no-op
			default:
				nameRuneValue, _ := utf8.DecodeRuneInString(name[0:])
				runeValue, _ := utf8.DecodeRuneInString(kind[0:])
				if unicode.IsLetter(runeValue) &&
					unicode.IsLetter(nameRuneValue) {
					file.TypeAliases = append(file.TypeAliases, types.TypeAlias{
						Ident: name,
						Alias: kind,
					})
				}
				// Ignore tokens: read type until end of line
				///*line := s.Line
				//for tok := s.Scan(); tok != scanner.EOF && line == s.Line; tok = s.Scan() {
				//	// scan until end of line
				//}*/
			}
		case "HRESULT":
			// Parse functions
			s.Scan()
			if tok := s.TokenText(); tok != "WINAPI" {
				// Ignore if not a function
				// Expecting pattern "HRESULT WINAPI"
				continue
			}
			s.Scan()
			funcName := s.TokenText()
			s.Scan()
			if tok := s.TokenText(); tok != "(" {
				panic(s.String() + ": unexpected token: " + tok + " after function name: " + funcName)
			}
			parameters := parseFunctionPointerParameterFields(&s)
			if tok := s.TokenText(); tok != ")" {
				panic(s.String() + ": unexpected token: " + tok + " after function parameters for: " + funcName)
			}
			s.Scan()
			if tok := s.TokenText(); tok != ";" {
				panic(s.String() + ": unexpected token: " + tok + " after function parameters for: " + funcName)
			}
			file.Functions = append(file.Functions, types.Function{
				Ident:      funcName,
				DLLCall:    funcName,
				Parameters: parameters,
			})
		case "interface":
			s.Scan()
			name := s.TokenText()
			s.Scan()
			if s.TokenText() == name {
				// ignore pattern "typedef interface ID3D11Device ID3D11Device;"
				continue
			}
			if tok := s.TokenText(); tok != "{" {
				panic(s.String() + ": unexpected token: " + tok + " for interface " + name)
			}
			data := types.Struct{
				Ident: name,
			}
			data.Fields = parseStructFields(&s)
			file.Structs = append(file.Structs, data)
		}
	}

	// Apply additional data
	for i := 0; i < len(file.Structs); i++ {
		record := &file.Structs[i]

		// Apply GUID data to struct (if it exists)
		if guid, ok := structIdentToGuid[record.Ident]; ok {
			record.GUID = guid
		}

		// NOTE(Jae): 2020-01-26
		// A bit of a hack to determine the name. Should probably make
		// this more robust but we'll see!
		determineVtblName := record.Ident + "Vtbl"
		if vtblStruct, ok := vtblStructIdentToData[determineVtblName]; ok {
			record.VtblStruct = vtblStruct
		}
	}

	return file
}

// parsePointerDepth will return 1 = *, 2 = **, 3 = ***, etc
func parsePointerDepth(s *scanner.Scanner) int {
	r := 0
	for t := s.TokenText(); t == "*"; t = s.TokenText() {
		s.Scan()
		r++
	}
	return r
}

func parseEnumExpr(s *scanner.Scanner, enumIdent string) (string, bool) {
	value := ""
	for {
		value += s.TokenText()
		s.Scan()
		switch tok := s.TokenText(); tok {
		case ",":
			return value, false
		case "}":
			return value, true
		}
	}
}

func tryEvaluateExpr(expr string) interface{} {
	if len(expr) >= 3 && expr[1] == 'x' {
		// Parse 0x1, 0x11, 0x1234, etc
		expr = expr[2:]
		if expr[len(expr)-1:] == "L" {
			expr = expr[:len(expr)-1]
		}
		switch len(expr) {
		case 1, 2:
			i, err := strconv.ParseUint(expr, 16, 8)
			if err != nil {
				panic(err)
			}
			return uint32(i)
		case 3, 4:
			i, err := strconv.ParseUint(expr, 16, 16)
			if err != nil {
				panic(err)
			}
			return uint32(i)
		case 5, 6, 7, 8:
			i, err := strconv.ParseUint(expr, 16, 32)
			if err != nil {
				panic(err)
			}
			return uint32(i)
		default:
			panic(fmt.Sprintf("Unhandld hex expression: %s, size is: %d", expr, len(expr)))
		}
		/*// uint32
		{
			i, err := strconv.ParseUint(expr, 10, 32)
			if err != nil {
				panic(err)
			}
			return uint32(i)
		}*/
	}
	return nil
}

func parseFunctionPointerParameterFields(s *scanner.Scanner) []types.StructField {
	return parseFields(s, ",", ")")
}

func parseStructFields(s *scanner.Scanner) []types.StructField {
	return parseFields(s, ";", "}")
}

func parseFields(s *scanner.Scanner, endOfFieldToken string, endOfListToken string) []types.StructField {
	var fields []types.StructField
FieldLoop:
	for {
		var isOut, isDeref, hasECount bool

		// Scan next field
		s.Scan()
		switch v := s.TokenText(); v {
		case endOfListToken:
			// End of struct ('}') or list (')')
			break FieldLoop
		case "BEGIN_INTERFACE":
			// The vtbl structs used in DirectX all seem to have the
			// macro BEGIN_INTERFACE in them.
			// We use this field to detect if the struct is a vtbl struct.
			fields = append(fields, types.StructField{
				Name: "BEGIN_INTERFACE",
			})
			continue
		case "END_INTERFACE":
			// Ignore END_INTERFACE macro
			continue
		case "union":
			s.Scan()
			if expect := "{"; s.TokenText() != expect {
				panic(s.String() + ": unexpected token: " + s.TokenText() + " expected " + expect + " after \"union\" keyword.")
			}

			// Parse union struct fields
			unionFields := parseStructFields(s)

			s.Scan()
			if expect := ";"; s.TokenText() != expect {
				panic(s.String() + ": unexpected token: " + s.TokenText() + " after " + expect + " union")
			}
			fields = append(fields, types.StructField{
				TypeInfo: types.NewUnion(types.Union{
					Fields: unionFields,
				}),
				Name:      "",
				IsOut:     isOut,
				HasECount: hasECount,
			})
			continue
		default:
			metaValue := s.TokenText()
			if strings.HasPrefix(metaValue, "__") {
				isOut = strings.Contains(metaValue, "_out")
				isDeref = strings.Contains(metaValue, "_deref")
				hasECount = strings.Contains(metaValue, "_ecount")

				// Skip meta info like:
				// - __in
				// - __in_bcount_opt( DataSize )
				s.Scan()
				if s.TokenText() == "(" {
					// Ignore params for now
					s.Scan()
					for depth := 0; ; {
						if s.TokenText() == ")" {
							if depth > 0 {
								depth--
							} else {
								s.Scan()
								break
							}
						}
						s.Scan()
						if s.TokenText() == "(" {
							depth++
							s.Scan()
						}
					}
				}
			}
		}
		// Read type info patterns:
		// - UINT
		// - CONST_VTBL struct
		// - const = const void
		kind := ""
		for {
			// TODO(Jae): maybe store pointer depth
			parsePointerDepth(s)
			v := s.TokenText()
			s.Scan()
			switch v {
			case "interface":
				// We skip this as we dont need this info to be captured for now
				//
				// Handle case in D3Dcompiler.h
				// - D3DDisassemble10Effect(__in interface ID3D10Effect *pEffect,
				continue
			case "const", "CONST":
				// Handle alternate "const" case where it
				// comes after the type rather than before
				// ie.
				// - "__in_ecount(NumBuffers)  ID3D11Buffer *const *ppConstantBuffers);"
				// - __in_ecount_opt( FeatureLevels ) CONST D3D_FEATURE_LEVEL* pFeatureLevels

				// TODO(Jae): 2020-01-26
				// Consider storing flag for const variable
				continue
			case "CONST_VTBL",
				"struct":
				if kind == "" {
					kind = v
				} else {
					kind = kind + " " + v
				}
				continue
			}
			if kind == "" {
				kind = v
			} else {
				kind = kind + " " + v
			}
			break
		}
		if s.TokenText() == "(" {
			// Detect function pointer
			s.Scan() // skip type, ie. STDMETHODCALLTYPE

			// TODO(Jae): Use pointer depth in return type
			s.Scan()
			parsePointerDepth(s)
			callType := s.TokenText()

			s.Scan()
			if s.TokenText() != ")" {
				panic(s.String() + ": unexpected token: " + s.TokenText() + " after type: " + kind)
			}
			// TODO(Jae):
			// Parse name of the field `HRESULT ( STDMETHODCALLTYPE *QueryInterface )`
			s.Scan()
			if s.TokenText() != "(" {
				panic(s.String() + ": unexpected token: " + s.TokenText() + " after type: " + kind)
			}
			params := parseFunctionPointerParameterFields(s)
			s.Scan()
			if s.TokenText() != ";" {
				panic(s.String() + ": unexpected token: " + s.TokenText() + " after type: " + kind)
			}
			fields = append(fields, types.StructField{
				TypeInfo: types.NewFunctionPointer(types.FunctionPointer{
					Parameters: params,
				}),
				Name:      callType,
				IsOut:     isOut,
				IsDeref:   isDeref,
				HasECount: hasECount,
			})
			continue
		}
		if s.TokenText() == "(" {
			panic(s.String() + ": unexpected token: " + s.TokenText() + " after type: " + kind)
		}
		// Get *const pointer or just * info
		pointerDepth := parsePointerDepth(s)
		switch v := s.TokenText(); v {
		case "const":
			// Handle alternate "const" case where it
			// comes after the type rather than before
			// ie.
			// - "__in_ecount(NumBuffers)  ID3D11Buffer *const *ppConstantBuffers);"
			s.Scan()

			// TODO(Jae): Flag this type as "const"?
			//kind = kind + " " + v

			// Re-read pointer info after *const
			pointerDepth += parsePointerDepth(s)
		}
		name := s.TokenText()
		//panic(s.String() + ": debug " + kind + " " + name)

		// Detect ;
		var typeInfo types.TypeInfo
		isLastField := false
		s.Scan()
		switch tok := s.TokenText(); tok {
		case endOfFieldToken, endOfListToken:
			// Simple type
			typeInfo = types.NewBasicType(kind, types.BasicType{})
			if builtInTypeTrans, ok := typetrans.BuiltInTypeTranslation(kind); ok {
				if builtInTypeTrans.Size == "ptr" {
					typeInfo.Name = builtInTypeTrans.GoType[1:]
					pointerDepth++
				}
			}
			isLastField = tok == endOfListToken
		case "[":
			// Array type
			var dimens []int
		ArrayLenLoop:
			for ; ; s.Scan() {
				switch tok := s.TokenText(); tok {
				case "[":
					s.Scan()
					{
						// Add
						dStr := s.TokenText()
						d, err := strconv.Atoi(dStr)
						if err != nil {
							panic(s.String() + ": cannot parse array len value: " + dStr + ", error: " + err.Error())
						}
						dimens = append(dimens, d)
					}
					s.Scan()
					if expect := "]"; s.TokenText() != expect {
						panic(s.String() + ": expected token: " + expect + " after array type: " + kind)
					}
				case endOfFieldToken:
					break ArrayLenLoop
				case endOfListToken:
					isLastField = true
					break ArrayLenLoop
				default:
					panic(s.String() + ": expected token: " + endOfFieldToken + " after array type: " + kind)
				}
			}
			typeInfo = types.NewArray(kind, types.Array{
				Dimens: dimens,
			})
		default:
			panic(s.String() + ": expected [ or " + endOfFieldToken + " token, mishandled token:" + name)
		}
		if pointerDepth > 0 {
			// Wrap in pointer type if applicable
			typeInfo = types.NewPointer(kind, types.Pointer{
				TypeInfo: typeInfo,
				Depth:    pointerDepth,
			})
		}
		fields = append(fields, types.StructField{
			TypeInfo:  typeInfo,
			Name:      name,
			IsOut:     isOut,
			IsDeref:   isDeref,
			HasECount: hasECount,
		})
		if isLastField {
			break FieldLoop
		}
	}
	return fields
}
