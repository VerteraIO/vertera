//go:build grpcgen

package controller

import (
	"context"
	"log"
	"net"
	"time"

	verterapb "github.com/VerteraIO/vertera/api/proto/v1"
	"github.com/VerteraIO/vertera/internal/controlplane/tasks"
	"github.com/VerteraIO/vertera/internal/controlplane/dispatch"
	"google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

type AgentServiceServer struct {
	verterapb.UnimplementedAgentServiceServer
}

// RunTLS starts the gRPC server with TLS credentials.
func RunTLS(addr string, creds credentials.TransportCredentials) error {
    lis, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    grpcServer := grpc.NewServer(grpc.Creds(creds))
    verterapb.RegisterAgentServiceServer(grpcServer, &AgentServiceServer{})
    log.Printf("gRPC controller (mTLS) listening on %s", addr)
    time.Sleep(5 * time.Millisecond)
    return grpcServer.Serve(lis)
}

func (s *AgentServiceServer) Register(ctx context.Context, req *verterapb.RegisterRequest) (*verterapb.RegisterResponse, error) {
	assigned := req.AgentId
	if assigned == "" {
		assigned = req.Hostname
	}
	log.Printf("agent registered: %s (host=%s)", assigned, req.Hostname)
	return &verterapb.RegisterResponse{AssignedId: assigned}, nil
}

func (s *AgentServiceServer) WatchTasks(req *verterapb.RegisterRequest, stream verterapb.AgentService_WatchTasksServer) error {
	hostID := req.AgentId
	if hostID == "" {
		hostID = req.Hostname
	}
	// First drain any pending tasks for this host
	pending := dispatch.Default.DrainPending(hostID)
	for _, t := range pending {
		pb := &verterapb.Task{Id: t.ID, HostId: t.HostID, Type: verterapb.TaskType_TASK_TYPE_INSTALL_PACKAGES, Params: t.Params}
		if err := stream.Send(pb); err != nil {
			return err
		}
	}
	// Subscribe for live tasks
	ch, unsubscribe := dispatch.Default.Subscribe(hostID)
	defer unsubscribe()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case t := <-ch:
			if t == nil { // channel closed
				return nil
			}
			pb := &verterapb.Task{Id: t.ID, HostId: t.HostID, Type: verterapb.TaskType_TASK_TYPE_INSTALL_PACKAGES, Params: t.Params}
			if err := stream.Send(pb); err != nil {
				return err
			}
		}
	}
}

func (s *AgentServiceServer) ReportTaskResult(ctx context.Context, result *verterapb.TaskResult) (*verterapb.TaskAck, error) {
    // Update in-memory task status
    if result.Logs != "" {
        tasks.Default.UpdateLogs(result.Id, result.Logs)
    }
    switch result.Status {
    case "running":
        tasks.Default.UpdateStatusRunning(result.Id)
    case "succeeded":
        tasks.Default.UpdateStatusSucceeded(result.Id)
	case "failed":
		msg := result.Error
		if msg == "" {
			msg = "unknown error"
		}
		tasks.Default.UpdateStatusFailed(result.Id, msg)
	default:
		log.Printf("ReportTaskResult: unknown status %q for task %s", result.Status, result.Id)
	}
	return &verterapb.TaskAck{Id: result.Id}, nil
}

// Run starts the gRPC server on addr (e.g., ":9090").
func Run(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	verterapb.RegisterAgentServiceServer(grpcServer, &AgentServiceServer{})
	log.Printf("gRPC controller listening on %s", addr)
	// Small delay to improve log interleaving during startup when run alongside HTTP
	time.Sleep(5 * time.Millisecond)
	return grpcServer.Serve(lis)
}
