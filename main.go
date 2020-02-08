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

	files := []string{
		"DXSDK_Jun10/include/D3D11.h",
		"DXSDK_Jun10/include/DXGI.h",
		"DXSDK_Jun10/include/DXGIType.h",
		"DXSDK_Jun10/include/D3Dcommon.h",
		"DXSDK_Jun10/include/DXGIFormat.h",
		"DXSDK_Jun10/include/D3D11SDKLayers.h",
		"DXSDK_Jun10/include/D3D11Shader.h",
	}

	// Get project
	var project types.Project
	{
		file := types.File{}
		file.Filename = "WINDOWS"
		file.TypeAliases = append(file.TypeAliases, []types.TypeAlias{
			{
				Ident: "HWND",
				Alias: "uintptr",
			},
			{
				Ident: "HMODULE",
				Alias: "uintptr",
			},
			/*{
				Ident: "BOOL",
				Alias: "uint32",
			},
			{
				Ident: "FLOAT",
				Alias: "float32",
			},*/
		}...)
		project.Files = append(project.Files, file)
	}
	for _, filename := range files {
		file := parser.ParseFile(filename)
		transformer.Transform(&file)
		project.Files = append(project.Files, file)
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

	// Add custom transforms later
	//for i := 0; i < len(project.Files); i++ {
	//	file := &project.Files[i]
	//	transformer.Transform(file)
	//}

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
