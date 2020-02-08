package main

import (
	"strings"
	"testing"
	"text/scanner"
)

type GoldenRule struct {
	In string
	//Out string
}

var structTestData = []GoldenRule{
	{
		// typedef struct D3D11_BOX
		In: `
	    UINT left;
	    UINT top;
	    UINT front;
	    UINT right;
	    UINT bottom;
	    UINT back;
	    } 	D3D11_BOX;`,
	},
	{
		// typedef struct ID3D11DeviceChildVtbl
		In: `
    	BEGIN_INTERFACE
        
        HRESULT ( STDMETHODCALLTYPE *QueryInterface )( 
            ID3D11DeviceChild * This,
            /* [in] */ REFIID riid,
            /* [annotation][iid_is][out] */ 
            __RPC__deref_out  void **ppvObject);`,
	},
}

func TestParseStruct(t *testing.T) {
	for _, test := range structTestData {
		var s scanner.Scanner
		s.Init(strings.NewReader(test.In))
		s.Filename = "Test"
		s.Mode = scanner.GoTokens
		parseStructFields(&s)
	}
}
