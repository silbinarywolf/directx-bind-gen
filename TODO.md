# Todo List

Personal and vague to-do list so I can keep track on what known-effort is remaining. This will change over time.

- Parse UUID from .h files
-- Add UUID to interfaces, stop using mocked uuid
- Add logic to change QueryInterface to follow this format
```
if err := device.QueryInterface(dxgiDevice.GUID(), &dxgiDevice); err != nil {
	panic(err)
}

//
func (obj *Device) QueryInterface(guid guid, ppvObject interface{}) (err Error) {
	v := reflect.ValueOf(ppvObject)
	switch v.Kind() {
	case reflect.Ptr:
		u := v.Pointer()
		ret, _, _ := syscall.Syscall(
			obj.lpVtbl.QueryInterface,
			3,
			uintptr(unsafe.Pointer(obj)),
			uintptr(unsafe.Pointer(&guid)),
			uintptr(unsafe.Pointer(u)),
		)
		err = toErr(ret)
	default:
		panic(`Unexpected ppvObject value. Example use:
var dxgiDevice *d3d11.IDXGIDevice
if err := device.QueryInterface(dxgiDevice.GUID(), &dxgiDevice); err != nil {
	panic(err)
}`)
	}
	return
}
```
- Add parsing for REGUID type that is a slice of type
-- guid REFGUID, pDataSize *uint32, pData uintptr
- Parse #define flags
-- ie. needed for setting: `typedef UINT DXGI_USAGE;`
- Figure out how to support structs with unions in Golang