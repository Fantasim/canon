package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
)

// message is any JSON-RPC 2.0 message the client sends: a request when ID is present, else a
// notification; one with no method is a response, which the server never asked for.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type resultResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   responseError   `json:"error"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// codeOf is the JSON-RPC code of err.
func codeOf(err error) int {
	for _, e := range errorCodes {
		if errors.Is(err, e.err) {
			return e.code
		}
	}
	return codeInternalError
}

// respond answers request id with result, or with err's code and text when err is not nil.
func (s *server) respond(id json.RawMessage, result any, err error) error {
	if err != nil {
		return s.send(errorResponse{JSONRPC: jsonrpcV2, ID: id, Error: responseError{Code: codeOf(err), Message: err.Error()}})
	}
	return s.send(resultResponse{JSONRPC: jsonrpcV2, ID: id, Result: result})
}

// notify sends a notification.
func (s *server) notify(method string, params any) error {
	return s.send(notification{JSONRPC: jsonrpcV2, Method: method, Params: params})
}

// send writes v as one message.
func (s *server) send(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf(fmtWrap, errWrite, err)
	}
	return s.out.write(body)
}

// decode reads params into v; malformed params are errInvalidParams.
func decode(params json.RawMessage, v any) error {
	if err := json.Unmarshal(params, v); err != nil {
		return fmt.Errorf(fmtWrap, errInvalidParams, err)
	}
	return nil
}
