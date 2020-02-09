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

	var file types.File
	file.Filename = filename

	var s scanner.Scanner
	s.Init(f)
	s.Filename = filename
	s.Mode = scanner.GoTokens //^= scanner.SkipComments // don't skip comments
	//MainLoop:
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "#":
			s.Scan()
			switch s.TokenText() {
			case "define":
				// todo(Jae): 2020-01-28
				// Store define constants...

				// A few cases to handle:
				// - #define _INCLUDE_H
				// - #define D3D_CONST ( 3 )
				// - #DEFINE DX_VERSION 455
				// Need to avoid consuming too many tokens ahead or else we accidentally
				// skip the D3D11_INPUT_ELEMENT_DESC struct

				// Skip until newline for now
				{
					oldMode := s.Mode
					oldWhitespace := s.Whitespace
					s.Mode ^= scanner.GoWhitespace
					// https://golang.org/pkg/text/scanner/#example__whitespace
					s.Whitespace ^= 1<<'\t' | 1<<'\n' // don't skip tabs and new lines
					for {
						s.Scan()
						v := s.TokenText()
						// For debugging
						// fmt.Printf("%v\n", []byte(v))
						if v == "" || v == "\n" {
							break
						}
					}
					s.Mode = oldMode
					s.Whitespace = oldWhitespace
				}
				//s.Scan()
				//macroName := s.TokenText()

				// do nothing with #define so far
				//fmt.Printf("def: %s %s\n", macroName, tok)
			}
			// Read macro until end of line
			// TODO(Jae): account for the newline backslash thing? ie.
			/*startLine := s.Line
			if s.lastLineLen > 0 {
				startLine++
			}
			for tok = s.Scan(); tok != scanner.EOF && line == s.Line; tok = s.Scan() {
				line := s.Scan
				if s.lastLineLen > 0 {

				}
				// scan until end of line
				//fmt.Printf("LINE111 tar: %d, actu: %d\n", line, s.Line)
			}*/
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
				data := types.Enum{}
				for {
					s.Scan()
					kind := s.TokenText()
					if kind == "}" {
						// This case occurs if enum has "," on last item
						break
					}
					s.Scan() // =
					if tok := s.TokenText(); tok != "=" {
						panic(s.String() + ": unexpected token: " + tok + " after enum field value: " + kind)
					}
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
						default:
							panic(fmt.Sprintf("Unhandled evaluated expression type: %T", value))
						}
					}
					data.Fields = append(data.Fields, enumField)
					if isEndOfEnum {
						break
					}
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
				data := types.Struct{
					Ident: name,
				}
				data.Fields = parseStructFields(&s)
				s.Scan() // Scan and get struct name again
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
	/*for i := 0; i < len(file.VtblStructs); i++ {
		record := &file.VtblStructs[i]
		guid, ok := structIdentToGuid[record.Ident]
		if !ok {
			fmt.Printf("%v\n", structIdentToGuid)
			panic(record.Ident)
			continue
		}
		record.GUID = guid
	}*/

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
		isOut := false
		hasECount := false

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
				HasECount: hasECount,
			})
			continue
		}
		if s.TokenText() == "(" {
			panic(s.String() + ": unexpected token: " + s.TokenText() + " after type: " + kind)
		}
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

			// Re-read pointer depth for the type
			pointerDepth = parsePointerDepth(s)
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
			HasECount: hasECount,
		})
		if isLastField {
			break FieldLoop
		}
	}
	return fields
}
