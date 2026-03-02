package gateway

import "net/http"

// handleChat delegates to the existing router.Handler.HandleChat.
func (s *Server) handleChat() http.HandlerFunc {
	return s.rh.HandleChat
}

// handleModels delegates to the existing router.Handler.HandleModels.
func (s *Server) handleModels() http.HandlerFunc {
	return s.rh.HandleModels
}
