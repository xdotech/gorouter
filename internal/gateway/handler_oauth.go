package gateway

import "net/http"

// handleOAuthAuthorize delegates to the oauth.Handler.
func (s *Server) handleOAuthAuthorize() http.HandlerFunc {
	return s.oh.Authorize
}

// handleOAuthCallback delegates to the oauth.Handler.
func (s *Server) handleOAuthCallback() http.HandlerFunc {
	return s.oh.Callback
}

// handleDeviceCode delegates to the oauth.Handler.
func (s *Server) handleDeviceCode() http.HandlerFunc {
	return s.oh.DeviceCode
}

// handlePoll delegates to the oauth.Handler.
func (s *Server) handlePoll() http.HandlerFunc {
	return s.oh.Poll
}
