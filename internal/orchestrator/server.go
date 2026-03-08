package orchestrator

import ("github.com/vky5/faultlab/internal/cluster" 
pb "github.com/vky5/faultlab/internal/protocol")



type Server struct {
	pb.UnimplementedOrchestratorServiceServer
	manager *cluster.Manager
}


func NewServer(m *cluster.Manager) *Server{
	return &Server{manager: m}
}