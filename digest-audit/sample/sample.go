package sample

import (
	"go.podman.io/image/v5/types"
)

// Global variable declarations - SHOULD be reported
var globalSysCtx types.SystemContext            // var definition
var globalSysCtxPtr *types.SystemContext        // var definition with pointer
var globalSysCtxPtr2 *types.SystemContext = nil // var definition + nil assignment

// Struct field declarations - SHOULD be reported
type Container struct {
	SysCtx    types.SystemContext  // field declaration - SHOULD be reported
	SysCtxPtr *types.SystemContext // field declaration (pointer) - SHOULD be reported
}

// Struct without SystemContext fields - should NOT be reported
type OtherContainer struct {
	Name string
}

// Function parameter declarations - should NOT be reported
func processContext(ctx *types.SystemContext) {
	// Parameter declaration is not reported
	_ = ctx
}

// Function taking SystemContext by value - parameter should NOT be reported
func processContextByValue(ctx types.SystemContext) {
	_ = ctx
}

// Return type - should NOT be reported
func getContext() *types.SystemContext {
	return nil // nil return - should be reported
}

// Return &types.SystemContext{} - SHOULD be reported
func getContextPtrEmpty() *types.SystemContext {
	return &types.SystemContext{} // empty literal ptr return - SHOULD be reported
}

// Return SystemContext by value
func getContextByValue() types.SystemContext {
	return types.SystemContext{} // empty literal return - SHOULD be reported
}

func CoverageFunction() {
	// Variable definitions via "var" - SHOULD be reported
	var ctx1 types.SystemContext  // var definition
	var ctx2 *types.SystemContext // var definition (pointer)

	// Variable definitions via ":=" - SHOULD be reported
	ctx3 := types.SystemContext{}  // := definition
	ctx4 := &types.SystemContext{} // := definition (pointer)

	// Setting variable to nil - SHOULD be reported
	ctx2 = nil // assignment to nil

	// Variable reassignment (not nil) - should NOT be reported
	ctx2 = &ctx1 // normal assignment (not nil)
	ctx2 = ctx4  // normal assignment (not nil)

	// Assigning SystemContext{} literal - SHOULD be reported (empty)
	ctx1 = types.SystemContext{} // literal assignment

	// Assigning SystemContext{...} literal with fields - SHOULD also be reported
	ctx1 = types.SystemContext{SystemRegistriesConfPath: "/etc"} // literal assignment (non-empty)

	// Assigning &types.SystemContext{} to pointer - SHOULD be reported
	ctx2 = &types.SystemContext{} // literal ptr assignment

	// Assigning &types.SystemContext{...} with fields - SHOULD also be reported
	ctx2 = &types.SystemContext{SystemRegistriesConfPath: "/etc"} // literal ptr assignment (non-empty)

	// Empty struct literal with *SystemContext field - SHOULD be reported (implicit nil)
	container := Container{}  // Container{} leaves SysCtxPtr implicitly nil
	container.SysCtxPtr = nil // field set to nil - SHOULD be reported

	// Setting field within SystemContext - should NOT be reported
	ctx3.SystemRegistriesConfPath = "/etc/containers/registries.conf" // field access

	// Passing nil as function argument - SHOULD be reported
	processContext(nil) // nil argument

	// Passing non-nil as function argument - should NOT be reported
	processContext(ctx2) // normal argument

	// Passing empty SystemContext{} literal as argument - SHOULD be reported
	processContextByValue(types.SystemContext{}) // empty literal argument

	// Passing &types.SystemContext{} as argument - SHOULD be reported
	processContext(&types.SystemContext{}) // empty literal ptr argument

	// Struct literal with nil field - SHOULD be reported
	_ = Container{
		SysCtxPtr: nil, // nil in struct literal
	}

	// Struct literal with empty SystemContext{} field - SHOULD be reported
	_ = Container{
		SysCtx: types.SystemContext{}, // empty literal in struct literal
	}

	// Struct literal with non-nil field - should NOT be reported
	_ = Container{
		SysCtxPtr: ctx2, // non-nil in struct literal
	}

	// Struct literal with &types.SystemContext{} field - SHOULD be reported
	_ = Container{
		SysCtxPtr: &types.SystemContext{}, // empty literal ptr in struct literal
	}

	// Struct literal with partial fields - SHOULD report implicit nil for missing *SystemContext field
	_ = Container{
		SysCtx: ctx1, // only sets SysCtx, leaves SysCtxPtr implicitly nil
	}

	// Struct literal for non-SystemContext struct - should NOT be reported
	_ = OtherContainer{}

	// Using context (not nil related) - should NOT be reported
	_ = ctx1
	_ = ctx2
	_ = ctx3
	_ = ctx4
}

// Method with receiver - should NOT be reported (function parameter)
func (c *Container) GetContext() *types.SystemContext {
	return c.SysCtxPtr // field access - should NOT be reported
}

// Type conversion of nil - should NOT be reported (not a function call)
var _ any = (*types.SystemContext)(nil)

// Multiple parameters - only function params should NOT be reported
func multiParam(a int, ctx *types.SystemContext, b string) {
	_ = a
	_ = ctx
	_ = b
}
