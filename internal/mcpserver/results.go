package mcpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func decodeArgs[T any](req *mcp.CallToolRequest) (T, error) {
	var input T
	raw := json.RawMessage(`{}`)
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		raw = req.Params.Arguments
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return input, errors.New("arguments must contain a single JSON object")
	}
	return input, nil
}

func toolSuccess(output any) (*mcp.CallToolResult, error) {
	return toolResult(output, false)
}

func toolError(code, message string) (*mcp.CallToolResult, error) {
	return toolResult(errorOutput{Error: code, Message: message}, true)
}

func toolResult(output any, isError bool) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: json.RawMessage(raw),
		IsError:           isError,
	}, nil
}
