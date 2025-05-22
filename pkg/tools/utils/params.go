package utils

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetStringParam safely extracts a string parameter from the request
func GetStringParam(req mcp.CallToolRequest, key string, required bool) (string, error) {
	val, exists := req.Params.Arguments[key]
	if !exists || val == nil {
		if required {
			return "", fmt.Errorf("missing required parameter: '%s'", key)
		}
		return "", nil
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("parameter '%s' must be a string", key)
	}

	return str, nil
}

// GetRequiredStringParam is a shorthand for GetStringParam with required=true
func GetRequiredStringParam(req mcp.CallToolRequest, key string) (string, error) {
	return GetStringParam(req, key, true)
}

// GetOptionalStringParam is a shorthand for GetStringParam with required=false
func GetOptionalStringParam(req mcp.CallToolRequest, key string) (string, error) {
	return GetStringParam(req, key, false)
}

// GetFloat64Param safely extracts a float64 parameter from the request
func GetFloat64Param(req mcp.CallToolRequest, key string, required bool) (float64, error) {
	val, exists := req.Params.Arguments[key]
	if !exists || val == nil {
		if required {
			return 0, fmt.Errorf("missing required parameter: '%s'", key)
		}
		return 0, nil
	}

	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("parameter '%s' must be a number", key)
	}

	return f, nil
}

// GetRequiredFloat64Param is a shorthand for GetFloat64Param with required=true
func GetRequiredFloat64Param(req mcp.CallToolRequest, key string) (float64, error) {
	return GetFloat64Param(req, key, true)
}

// GetOptionalFloat64Param is a shorthand for GetFloat64Param with required=false
func GetOptionalFloat64Param(req mcp.CallToolRequest, key string) (float64, error) {
	return GetFloat64Param(req, key, false)
}

// GetIntParam safely extracts an int parameter from a float64 in the request
func GetIntParam(req mcp.CallToolRequest, key string, required bool) (int, error) {
	f, err := GetFloat64Param(req, key, required)
	if err != nil {
		return 0, err
	}

	if f == 0 && !required {
		return 0, nil
	}

	return int(f), nil
}

// GetRequiredIntParam is a shorthand for GetIntParam with required=true
func GetRequiredIntParam(req mcp.CallToolRequest, key string) (int, error) {
	return GetIntParam(req, key, true)
}

// GetOptionalIntParam is a shorthand for GetIntParam with required=false
func GetOptionalIntParam(req mcp.CallToolRequest, key string) (int, error) {
	return GetIntParam(req, key, false)
}

// GetBoolParam safely extracts a bool parameter from the request
func GetBoolParam(req mcp.CallToolRequest, key string, required bool) (bool, error) {
	val, exists := req.Params.Arguments[key]
	if !exists || val == nil {
		if required {
			return false, fmt.Errorf("missing required parameter: '%s'", key)
		}
		return false, nil
	}

	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("parameter '%s' must be a boolean", key)
	}

	return b, nil
}

// GetRequiredBoolParam is a shorthand for GetBoolParam with required=true
func GetRequiredBoolParam(req mcp.CallToolRequest, key string) (bool, error) {
	return GetBoolParam(req, key, true)
}

// GetOptionalBoolParam is a shorthand for GetBoolParam with required=false
func GetOptionalBoolParam(req mcp.CallToolRequest, key string) (bool, error) {
	return GetBoolParam(req, key, false)
}

// GetMapParam safely extracts a map parameter from the request
func GetMapParam(req mcp.CallToolRequest, key string, required bool) (map[string]any, error) {
	val, exists := req.Params.Arguments[key]
	if !exists || val == nil {
		if required {
			return nil, fmt.Errorf("missing required parameter: '%s'", key)
		}
		return nil, nil
	}

	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be an object", key)
	}

	return m, nil
}

// GetRequiredMapParam is a shorthand for GetMapParam with required=true
func GetRequiredMapParam(req mcp.CallToolRequest, key string) (map[string]any, error) {
	return GetMapParam(req, key, true)
}

// GetOptionalMapParam is a shorthand for GetMapParam with required=false
func GetOptionalMapParam(req mcp.CallToolRequest, key string) (map[string]any, error) {
	return GetMapParam(req, key, false)
}

// GetArrayParam safely extracts an array parameter from the request
func GetArrayParam(req mcp.CallToolRequest, key string, required bool) ([]any, error) {
	val, exists := req.Params.Arguments[key]
	if !exists || val == nil {
		if required {
			return nil, fmt.Errorf("missing required parameter: '%s'", key)
		}
		return nil, nil
	}

	arr, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be an array", key)
	}

	return arr, nil
}

// GetRequiredArrayParam is a shorthand for GetArrayParam with required=true
func GetRequiredArrayParam(req mcp.CallToolRequest, key string) ([]any, error) {
	return GetArrayParam(req, key, true)
}

// GetOptionalArrayParam is a shorthand for GetArrayParam with required=false
func GetOptionalArrayParam(req mcp.CallToolRequest, key string) ([]any, error) {
	return GetArrayParam(req, key, false)
}

// HandleParameterError returns a properly formatted error response for parameter validation errors
func HandleParameterError(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}
