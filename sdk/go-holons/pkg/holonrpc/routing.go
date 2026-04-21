package holonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	routeModeDefault           = ""
	routeModeBroadcastResponse = "broadcast-response"
	routeModeFullBroadcast     = "full-broadcast"
)

type routeHints struct {
	targetPeerID string
	mode         string
}

type fanoutResult struct {
	Peer   string         `json:"peer"`
	Result map[string]any `json:"result,omitempty"`
	Error  *ResponseError `json:"error,omitempty"`
}

func parseRouteHints(method string, params map[string]any) (string, bool, map[string]any, routeHints, error) {
	trimmedMethod := strings.TrimSpace(method)
	if trimmedMethod == "" {
		return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidRequest, Message: "invalid request"}
	}

	cleaned := make(map[string]any, len(params))
	for k, v := range params {
		cleaned[k] = v
	}

	rh := routeHints{mode: routeModeDefault}
	if rawRouting, ok := cleaned["_routing"]; ok {
		mode, ok := rawRouting.(string)
		if !ok {
			return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidParams, Message: "_routing must be a string"}
		}
		mode = strings.TrimSpace(mode)
		switch mode {
		case routeModeDefault, routeModeBroadcastResponse, routeModeFullBroadcast:
			rh.mode = mode
		default:
			return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidParams, Message: fmt.Sprintf("unsupported _routing %q", mode)}
		}
		delete(cleaned, "_routing")
	}

	if rawPeer, ok := cleaned["_peer"]; ok {
		peerID, ok := rawPeer.(string)
		if !ok {
			return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidParams, Message: "_peer must be a string"}
		}
		peerID = strings.TrimSpace(peerID)
		if peerID == "" {
			return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidParams, Message: "_peer must be non-empty"}
		}
		rh.targetPeerID = peerID
		delete(cleaned, "_peer")
	}

	fanOut := strings.HasPrefix(trimmedMethod, "*.")
	if fanOut {
		trimmedMethod = strings.TrimPrefix(trimmedMethod, "*.")
		trimmedMethod = strings.TrimSpace(trimmedMethod)
		if trimmedMethod == "" {
			return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidRequest, Message: "invalid fan-out method"}
		}
	}

	if rh.mode == routeModeFullBroadcast && !fanOut {
		return "", false, nil, routeHints{}, &ResponseError{Code: codeInvalidParams, Message: "full-broadcast requires a fan-out method"}
	}

	return trimmedMethod, fanOut, cleaned, rh, nil
}

func (s *Server) routePeerRequest(caller *serverPeer, reqID json.RawMessage, method string, params map[string]any, hints routeHints, fanOut bool) (bool, error) {
	if fanOut {
		entries, err := s.dispatchFanOut(caller, method, params)
		if err != nil {
			if hasID(reqID) {
				return true, s.sendRPCError(caller, reqID, err)
			}
			return true, nil
		}

		if hints.mode == routeModeFullBroadcast {
			for _, entry := range entries {
				payload := map[string]any{"peer": entry.Peer}
				if entry.Error != nil {
					payload["error"] = map[string]any{
						"code":    entry.Error.Code,
						"message": entry.Error.Message,
						"data":    entry.Error.Data,
					}
				} else {
					payload["result"] = entry.Result
				}
				s.broadcastNotificationMany(map[string]struct{}{caller.id: {}, entry.Peer: {}}, method, payload)
			}
		}

		if hasID(reqID) {
			return true, s.sendPeerResultAny(caller, reqID, entries)
		}
		return true, nil
	}

	if hints.targetPeerID == "" {
		return false, nil
	}
	if !s.peerExists(hints.targetPeerID) {
		err := &ResponseError{Code: 5, Message: fmt.Sprintf("peer %q not found", hints.targetPeerID)}
		if hasID(reqID) {
			return true, s.sendRPCError(caller, reqID, err)
		}
		return true, nil
	}

	result, err := s.Invoke(caller.ctx, hints.targetPeerID, method, params)
	if err != nil {
		if hasID(reqID) {
			return true, s.sendRPCError(caller, reqID, err)
		}
		return true, nil
	}

	if hints.mode == routeModeBroadcastResponse {
		s.broadcastNotificationMany(
			map[string]struct{}{caller.id: {}, hints.targetPeerID: {}},
			method,
			map[string]any{"peer": hints.targetPeerID, "result": result},
		)
	}

	if hasID(reqID) {
		return true, s.sendPeerResult(caller, reqID, result)
	}
	return true, nil
}

func (s *Server) dispatchFanOut(caller *serverPeer, method string, params map[string]any) ([]fanoutResult, error) {
	targets := s.snapshotPeerIDsExcluding(caller.id)
	if len(targets) == 0 {
		return nil, &ResponseError{Code: 5, Message: "no connected peers"}
	}

	resultsCh := make(chan fanoutResult, len(targets))
	var wg sync.WaitGroup
	wg.Add(len(targets))

	for _, targetID := range targets {
		targetID := targetID
		go func() {
			defer wg.Done()

			out, err := s.Invoke(caller.ctx, targetID, method, params)
			entry := fanoutResult{Peer: targetID}
			if err != nil {
				entry.Error = toResponseError(err)
			} else {
				entry.Result = out
			}
			resultsCh <- entry
		}()
	}

	wg.Wait()
	close(resultsCh)

	entries := make([]fanoutResult, 0, len(targets))
	for entry := range resultsCh {
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *Server) snapshotPeerIDsExcluding(excludedPeerID string) []string {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	ids := make([]string, 0, len(s.peers))
	for peerID := range s.peers {
		if peerID == excludedPeerID {
			continue
		}
		ids = append(ids, peerID)
	}
	return ids
}

func (s *Server) peerExists(peerID string) bool {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()
	_, ok := s.peers[peerID]
	return ok
}

func (s *Server) broadcastNotification(excludePeerID, method string, payload map[string]any) {
	exclude := map[string]struct{}{}
	if excludePeerID != "" {
		exclude[excludePeerID] = struct{}{}
	}
	s.broadcastNotificationMany(exclude, method, payload)
}

func (s *Server) broadcastNotificationMany(exclude map[string]struct{}, method string, payload map[string]any) {
	paramsRaw, err := marshalObject(payload)
	if err != nil {
		return
	}

	msg, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  paramsRaw,
	})
	if err != nil {
		return
	}

	s.peersMu.RLock()
	peers := make([]*serverPeer, 0, len(s.peers))
	for peerID, peer := range s.peers {
		if _, skipped := exclude[peerID]; skipped {
			continue
		}
		peers = append(peers, peer)
	}
	s.peersMu.RUnlock()

	for _, peer := range peers {
		_ = s.writePeer(peer, msg)
	}
}

func (s *Server) sendPeerResultAny(peer *serverPeer, id json.RawMessage, result any) error {
	resultRaw, err := marshalAnyResult(result)
	if err != nil {
		// Framework-level marshal failure → JSON-RPC internal error (§5.2).
		return s.sendPeerError(peer, id, codeInternalError, "internal error", nil)
	}

	msg, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  resultRaw,
	})
	if err != nil {
		return s.sendPeerError(peer, id, codeInternalError, "internal error", nil)
	}
	return s.writePeer(peer, msg)
}

func marshalAnyResult(result any) (json.RawMessage, error) {
	if result == nil {
		return json.RawMessage("{}"), nil
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func (s *Server) sendRPCError(peer *serverPeer, reqID json.RawMessage, err error) error {
	rpcErr := toResponseError(err)
	return s.sendPeerError(peer, reqID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
}

func toResponseError(err error) *ResponseError {
	if err == nil {
		return nil
	}

	var rpcErr *ResponseError
	if errors.As(err, &rpcErr) {
		return rpcErr
	}

	switch {
	case errors.Is(err, context.Canceled):
		return &ResponseError{Code: 1, Message: err.Error()}
	case errors.Is(err, context.DeadlineExceeded):
		return &ResponseError{Code: 4, Message: err.Error()}
	default:
		return &ResponseError{Code: codeUnavailable, Message: err.Error()}
	}
}
