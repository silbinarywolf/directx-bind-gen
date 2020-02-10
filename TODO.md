# Todo List

Personal and vague to-do list so I can keep track on what known-effort is remaining. This will change over time.

- Parse #define flags
-- need to get working: #define DXGI_USAGE_RENDER_TARGET_OUTPUT     ( 1L << (1 + 4) )
-- ie. needed for setting: `typedef UINT DXGI_USAGE;`
- Add parsing for REGUID type that is a slice of type
-- guid REFGUID, pDataSize *uint32, pData uintptr
- Figure out how to support structs with unions in Golang
