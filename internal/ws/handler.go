package ws

// RegisterAllHandlers wires up every WS handler by delegating to per-domain
// registration functions. Each domain lives in its own handlers_<domain>.go
// file to keep this package aligned with the httpapi module split.
func (s *Server) RegisterAllHandlers() {
	s.RegisterAuthHandlers()
	s.RegisterUserHandlers()
	s.RegisterRoleHandlers()
	s.RegisterItemHandlers()
	s.RegisterSocialHandlers()
	s.RegisterTutorialHandlers()
	s.RegisterDecorationHandlers()
	s.RegisterSideGameHandlers()
	s.RegisterBustProtectHandlers()
	s.RegisterHeartBeatHandlers()
	s.RegisterLobbyHandlers()
	s.RegisterTournamentHandlers()
	s.RegisterRoomHandlers()
	s.RegisterPokerHandlers()
}
