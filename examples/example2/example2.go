package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"

	"github.com/gonutz/w32"
	d3d11 "github.com/silbinarywolf/directx-bind-gen/dist"
)

var _ d3d11.DXGI_FORMAT

type messageCallback func(window w32.HWND, msg uint32, w, l uintptr) uintptr

const (
	windowWidth  = 1280
	windowHeight = 720
)

func init() {
	runtime.LockOSThread()
}

// Example is a work-in-progress translation of DirectX11 tutorials
func Example() {
	var window w32.HWND
	{
		var err error
		window, err = openWindow("class name", handleEvent, 0, 0, windowWidth, windowHeight)
		if err != nil {
			panic(err)
		}
		w32.SetWindowText(window, "My New Window")
		w32.ShowCursor(false)
		w32.ShowWindow(window, 1)
	}
	driverTypes := []d3d11.DRIVER_TYPE{
		d3d11.DRIVER_TYPE_HARDWARE,
		d3d11.DRIVER_TYPE_WARP,
		d3d11.DRIVER_TYPE_REFERENCE,
	}
	featureLevels := []d3d11.FEATURE_LEVEL{
		//d3d11.FEATURE_LEVEL_11_1,
		d3d11.FEATURE_LEVEL_11_0,
		d3d11.FEATURE_LEVEL_10_1,
		d3d11.FEATURE_LEVEL_10_0,
	}
	var (
		device           *d3d11.Device
		featureLevel     d3d11.FEATURE_LEVEL
		immediateContext *d3d11.DeviceContext
		err              d3d11.Error
	)
	driverType := d3d11.DRIVER_TYPE_NULL
	for driverTypeIndex, _ := range driverTypes {
		driverType = driverTypes[driverTypeIndex]
		device, featureLevel, immediateContext, err = d3d11.CreateDevice(
			nil,
			driverType,
			0,
			uint32(d3d11.CREATE_DEVICE_DEBUG),
			featureLevels,
			d3d11.SDK_VERSION,
		)
		if err != nil {
			if err.Code() != d3d11.E_INVALIDARG {
				panic(err)
			}
			//panic(err)
		}
	}
	if err != nil {
		panic(err)
	}

	// Obtain DXGI factory from device (since we used 0 for adapter above)
	var dxgiFactory *d3d11.IDXGIFactory1
	{
		var dxgiDevice *d3d11.IDXGIDevice
		if err := device.QueryInterface(dxgiDevice.GUID(), &dxgiDevice); err != nil {
			panic(err)
		}
		adapter, err := dxgiDevice.GetAdapter()
		if err != nil {
			panic(err)
		}
		if err := adapter.GetParent(dxgiFactory.GUID(), &dxgiFactory); err != nil {
			panic(err)
		}
		adapter.Release()
		dxgiDevice.Release()
		if err != nil {
			panic(err)
		}
		fmt.Printf(`
			IDXGIDevice: %v
			Adapter: %v
			dxgiFactory: %v
			Error: %v
		`, dxgiDevice, adapter, dxgiFactory, err)
	}

	fmt.Printf(`
		Device: %v
		featureLevel: %v
		immediateContext: %v
		err: %v
	`, device, featureLevel, immediateContext, err)

	// DirectX 11.0 systems
	var swapChain *d3d11.IDXGISwapChain
	{
		sd := d3d11.DXGI_SWAP_CHAIN_DESC{}
		sd.BufferCount = 1
		sd.BufferDesc.Width = windowWidth
		sd.BufferDesc.Height = windowHeight
		sd.BufferDesc.Format = d3d11.DXGI_FORMAT_R8G8B8A8_UNORM
		sd.BufferDesc.RefreshRate.Numerator = 60
		sd.BufferDesc.RefreshRate.Denominator = 1
		sd.BufferUsage = d3d11.DXGI_USAGE_RENDER_TARGET_OUTPUT
		sd.OutputWindow = d3d11.HWND(window)
		sd.SampleDesc.Count = 1
		sd.SampleDesc.Quality = 0
		sd.Windowed = 1
		swapChain, err = dxgiFactory.CreateSwapChain(device, &sd)
		if err != nil {
			panic(err.Error())
		}
	}

	// Note this tutorial doesn't handle full-screen swapchains so we block the ALT+ENTER shortcut
	dxgiFactory.MakeWindowAssociation(d3d11.HWND(window), d3d11.DXGI_MWA_NO_ALT_ENTER)
	dxgiFactory.Release()

	var backBuffer *d3d11.Texture2D
	if err := swapChain.GetBuffer(0, backBuffer.GUID(), &backBuffer); err != nil {
		panic(err.Error())
	}
	renderTargetView, err := device.CreateRenderTargetView(backBuffer, nil)
	backBuffer.Release()
	if err != nil {
		panic(err.Error())
	}
	immediateContext.OMSetRenderTargets(1, &renderTargetView, nil)
	viewport := d3d11.VIEWPORT{
		Width:    windowWidth,
		Height:   windowHeight,
		MinDepth: 0.0,
		MaxDepth: 0.0,
		TopLeftX: 0,
		TopLeftY: 0,
	}
	immediateContext.RSSetViewports([]d3d11.VIEWPORT{viewport})
	fmt.Printf(`
		renderTargetView: %v
		viewports: %v
	`, renderTargetView, viewport)

	// Compile the vertex shader

	// Compile the vertex shader
	/*ID3DBlob* pVSBlob = nullptr;
	    hr = CompileShaderFromFile( L"Tutorial02.fx", "VS", "vs_4_0", &pVSBlob );
	    if( FAILED( hr ) )
	    {
	        MessageBox( nullptr,
	                    L"The FX file cannot be compiled.  Please run this executable from the directory that contains the FX file.", L"Error", MB_OK );
	        return hr;
	    }

		// Create the vertex shader
		hr = g_pd3dDevice->CreateVertexShader( pVSBlob->GetBufferPointer(), pVSBlob->GetBufferSize(), nullptr, &g_pVertexShader );
		if( FAILED( hr ) )
		{
			pVSBlob->Release();
	        return hr;
		}

	    // Define the input layout
	    D3D11_INPUT_ELEMENT_DESC layout[] =
	    {
	        { "POSITION", 0, DXGI_FORMAT_R32G32B32_FLOAT, 0, 0, D3D11_INPUT_PER_VERTEX_DATA, 0 },
	    };
		UINT numElements = ARRAYSIZE( layout );

	    // Create the input layout
		hr = g_pd3dDevice->CreateInputLayout( layout, numElements, pVSBlob->GetBufferPointer(),
	                                          pVSBlob->GetBufferSize(), &g_pVertexLayout );
		pVSBlob->Release();
		if( FAILED( hr ) )
	        return hr;

	    // Set the input layout
	    g_pImmediateContext->IASetInputLayout( g_pVertexLayout );

		// Compile the pixel shader
		ID3DBlob* pPSBlob = nullptr;
	    hr = CompileShaderFromFile( L"Tutorial02.fx", "PS", "ps_4_0", &pPSBlob );
	    if( FAILED( hr ) )
	    {
	        MessageBox( nullptr,
	                    L"The FX file cannot be compiled.  Please run this executable from the directory that contains the FX file.", L"Error", MB_OK );
	        return hr;
	    }

		// Create the pixel shader
		hr = g_pd3dDevice->CreatePixelShader( pPSBlob->GetBufferPointer(), pPSBlob->GetBufferSize(), nullptr, &g_pPixelShader );
		pPSBlob->Release();
	    if( FAILED( hr ) )
	        return hr;

	    // Create vertex buffer
	    SimpleVertex vertices[] =
	    {
	        XMFLOAT3( 0.0f, 0.5f, 0.5f ),
	        XMFLOAT3( 0.5f, -0.5f, 0.5f ),
	        XMFLOAT3( -0.5f, -0.5f, 0.5f ),
	    };
	    D3D11_BUFFER_DESC bd = {};
	    bd.Usage = D3D11_USAGE_DEFAULT;
	    bd.ByteWidth = sizeof( SimpleVertex ) * 3;
	    bd.BindFlags = D3D11_BIND_VERTEX_BUFFER;
		bd.CPUAccessFlags = 0;

	    D3D11_SUBRESOURCE_DATA InitData = {};
	    InitData.pSysMem = vertices;
	    hr = g_pd3dDevice->CreateBuffer( &bd, &InitData, &g_pVertexBuffer );
	    if( FAILED( hr ) )
	        return hr;

	    // Set vertex buffer
	    UINT stride = sizeof( SimpleVertex );
	    UINT offset = 0;
	    g_pImmediateContext->IASetVertexBuffers( 0, 1, &g_pVertexBuffer, &stride, &offset );

	    // Set primitive topology
	    g_pImmediateContext->IASetPrimitiveTopology( D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST );*/

	//
	var msg w32.MSG
	for msg.Message != w32.WM_QUIT {
		if w32.PeekMessage(&msg, 0, 0, 0, w32.PM_REMOVE) {
			w32.TranslateMessage(&msg)
			w32.DispatchMessage(&msg)
			fmt.Printf("message %d\n", msg.Message)
		} else {
			// Just clear the backbuffer
			var midnightBlue = [4]float32{0.098039225, 0.098039225, 0.439215720, 1.000000000}
			immediateContext.ClearRenderTargetView(renderTargetView, midnightBlue)
			swapChain.Present(0, 0)
			fmt.Printf("render\n")
		}
	}
}

func openWindow(
	className string,
	callback messageCallback,
	x, y, width, height int,
) (w32.HWND, error) {
	windowProc := syscall.NewCallback(callback)

	class := w32.WNDCLASSEX{
		WndProc:   windowProc,
		Cursor:    w32.LoadCursor(0, w32.MakeIntResource(w32.IDC_ARROW)),
		ClassName: syscall.StringToUTF16Ptr(className),
	}
	if w32.RegisterClassEx(&class) == 0 {
		return 0, errors.New("RegisterClassEx failed")
	}

	window := w32.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr(className),
		nil,
		w32.WS_OVERLAPPEDWINDOW|w32.WS_VISIBLE,
		x, y, width, height,
		0, 0, 0, nil,
	)
	if window == 0 {
		return 0, errors.New("CreateWindowEx failed")
	}

	return window, nil
}

func compileShaderFromFile(filename string, entryPoint string, shaderModel string) *d3d11.Blob {
	shaderFlags := d3d11.D3DCOMPILE_ENABLE_STRICTNESS

	// Debug
	shaderFlags |= d3d11.D3DCOMPILE_DEBUG

	// Debug: Disable optimizations to further improve shader debugging
	shaderFlags |= d3d11.D3DCOMPILE_SKIP_OPTIMIZATION

	var errorBlob *d3d11.Blob
	err := d3d11.D3DCompileFromFile(szFileName, nullptr, nullptr, szEntryPoint, szShaderModel, shaderFlags, 0, ppBlobOut, &pErrorBlob)
	if err != nil {
		panic(err)
	}
	return errorBlob
	/*
		HRESULT CompileShaderFromFile( const WCHAR* szFileName, LPCSTR szEntryPoint, LPCSTR szShaderModel, ID3DBlob** ppBlobOut )
		{
		    HRESULT hr = S_OK;

		    DWORD dwShaderFlags = D3DCOMPILE_ENABLE_STRICTNESS;
		#ifdef _DEBUG
		    // Set the D3DCOMPILE_DEBUG flag to embed debug information in the shaders.
		    // Setting this flag improves the shader debugging experience, but still allows
		    // the shaders to be optimized and to run exactly the way they will run in
		    // the release configuration of this program.
		    dwShaderFlags |= D3DCOMPILE_DEBUG;

		    // Disable optimizations to further improve shader debugging
		    dwShaderFlags |= D3DCOMPILE_SKIP_OPTIMIZATION;
		#endif

		    ID3DBlob* pErrorBlob = nullptr;
		    hr = D3DCompileFromFile( szFileName, nullptr, nullptr, szEntryPoint, szShaderModel,
		        dwShaderFlags, 0, ppBlobOut, &pErrorBlob );
		    if( FAILED(hr) )
		    {
		        if( pErrorBlob )
		        {
		            OutputDebugStringA( reinterpret_cast<const char*>( pErrorBlob->GetBufferPointer() ) );
		            pErrorBlob->Release();
		        }
		        return hr;
		    }
		    if( pErrorBlob ) pErrorBlob->Release();

		    return S_OK;
		}
	*/
}

func handleEvent(window w32.HWND, message uint32, w, l uintptr) uintptr {
	switch message {
	case w32.WM_KEYDOWN:
		if !isKeyRepeat(l) {
			switch w {
			}
		}
		return 1
	case w32.WM_KEYUP:
		if !isKeyRepeat(l) {
			switch w {
			}
		}
		return 1
	case w32.WM_SIZE:
		return 1
	case w32.WM_DESTROY:
		w32.PostQuitMessage(0)
		return 1
	default:
		return w32.DefWindowProc(window, message, w, l)
	}
}

func isKeyRepeat(l uintptr) bool {
	return l&(1<<30) != 0
}

func main() {
	Example()
}
