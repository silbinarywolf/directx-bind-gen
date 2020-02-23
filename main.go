package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/silbinarywolf/directx-bind-gen/internal/parser"
	"github.com/silbinarywolf/directx-bind-gen/internal/printer"
	"github.com/silbinarywolf/directx-bind-gen/internal/transformer"
	"github.com/silbinarywolf/directx-bind-gen/internal/types"
)

func main() {
	filename := "DXSDK_Jun10/include/D3D11.h"
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	const dir = "DXSDK_Jun10/include/"
	files := []string{
		"D3D11.h",
		"DXGI.h",
		"DXGIType.h",
		"D3Dcommon.h",
		"DXGIFormat.h",
		"D3D11SDKLayers.h",
		"D3D11Shader.h",
		"D3Dcompiler.h",
	}

	// Get project
	var project types.Project
	{
		file := types.File{}
		file.Filename = "directx-bind-gen"
		file.TypeAliases = append(file.TypeAliases, []types.TypeAlias{
			{
				Ident: "HWND",
				Alias: "uintptr",
			},
			{
				Ident: "HMODULE",
				Alias: "uintptr",
			},
			{
				// Not sure how to best handle this, so going to assume it takes any pointer for now
				//DECLARE_INTERFACE(ID3DInclude)
				//{
				//  STDMETHOD(Open)(THIS_ D3D_INCLUDE_TYPE IncludeType, LPCSTR pFileName, LPCVOID pParentData, LPCVOID *ppData, UINT *pBytes) PURE;
				//  STDMETHOD(Close)(THIS_ LPCVOID pData) PURE;
				//};
				Ident: "ID3DInclude",
				Alias: "uintptr",
			},
		}...)
		file.Structs = append(file.Structs, []types.Struct{
			// Provide data types from Windows APIs
			{
				// guid describes a structure used to describe an identifier for a MAPI interface.
				// https://docs.microsoft.com/en-us/office/client-developer/outlook/mapi/iid
				//
				// implemented here too: https://github.com/golang/sys/blob/master/windows/types_windows.go#L1216
				Ident: "GUID",
				Fields: []types.StructField{
					{
						Name:     "Data1",
						TypeInfo: types.NewBasicType("uint32", types.BasicType{}),
					},
					{
						Name:     "Data2",
						TypeInfo: types.NewBasicType("uint16", types.BasicType{}),
					},
					{
						Name:     "Data3",
						TypeInfo: types.NewBasicType("uint16", types.BasicType{}),
					},
					{
						Name: "Data4",
						TypeInfo: types.NewArray("byte", types.Array{
							Dimens: []int{8},
						}),
					},
				},
			},
			{
				// Rect structure defines a rectangle by the coordinates of its upper-left and lower-right corners.
				// https://docs.microsoft.com/en-us/windows/win32/api/windef/ns-windef-rect
				Ident: "Rect",
				Fields: []types.StructField{
					{
						Name:     "Left",
						TypeInfo: types.NewBasicType("int32", types.BasicType{}),
					},
					{
						Name:     "Top",
						TypeInfo: types.NewBasicType("int32", types.BasicType{}),
					},
					{
						Name:     "Right",
						TypeInfo: types.NewBasicType("int32", types.BasicType{}),
					},
					{
						Name:     "Bottom",
						TypeInfo: types.NewBasicType("int32", types.BasicType{}),
					},
				},
			},
		}...)
		file.Macros = append(file.Macros, []types.Macro{
			{
				// E_INVALIDARG indicates that an invalid parameter was passed to the
				// returning function.
				Ident: "E_INVALIDARG",
				Value: types.Value{
					RawValue: "-2147024809",
				},
			},
		}...)
		project.Files = append(project.Files, file)
	}
	for _, filename := range files {
		filename = dir + filename
		file := parser.ParseFile(filename)
		project.Files = append(project.Files, file)
	}

	// Perform customised transforms
	for i := 0; i < len(project.Files); i++ {
		file := &project.Files[i]
		transformer.Transform(file)
	}

	// Output JSON
	{
		outputFolderName := "data"
		for _, file := range project.Files {
			if file.Filename == "" {
				panic("Missing Filename.")
			}
			res, err := json.MarshalIndent(file, "", "  ")
			if err != nil {
				panic(err)
			}
			baseName := filepath.Base(file.Filename)
			baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
			outputPath := outputFolderName + "/" + baseName + ".json"
			if err := ioutil.WriteFile(outputPath, res, 0644); err != nil {
				panic(err)
			}
		}
	}

	// Create Golang bindings
	{
		outputData := printer.PrintProject(&project)
		if _, err := os.Stat("dist"); err != nil {
			if err := os.Mkdir("dist", 0777); err != nil {
				panic(err)
			}
		}
		err = ioutil.WriteFile("dist/d3d11.go", outputData, 0644)
		if err != nil {
			panic(err)
		}
	}
}
