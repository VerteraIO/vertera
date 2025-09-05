//go:build grpcgen

package agent

import (
	"context"
	"log"
	"encoding/json"
	"os"
	"path/filepath"

	verterapb "github.com/VerteraIO/vertera/api/proto/v1"
	"google.golang.org/grpc"
	"github.com/VerteraIO/vertera/internal/packages"
)

// Run connects to the controller and watches for tasks.
// Optional dial options can be provided (e.g., grpc.WithTransportCredentials()).
func Run(ctx context.Context, addr, agentID, hostname string, dialOpts ...grpc.DialOption) error {
	if len(dialOpts) == 0 {
		dialOpts = []grpc.DialOption{grpc.WithInsecure()}
	}
	conn, err := grpc.DialContext(ctx, addr, append(dialOpts, grpc.WithBlock())...)
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := verterapb.NewAgentServiceClient(conn)

	// Register
	reg := &verterapb.RegisterRequest{AgentId: agentID, Hostname: hostname}
	if _, err := cli.Register(ctx, reg); err != nil {
		return err
	}
	log.Printf("agent registered id=%s host=%s", agentID, hostname)

	// Watch tasks (placeholder)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := cli.WatchTasks(ctx, reg)
	if err != nil {
		return err
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		log.Printf("received task: %s type=%v", msg.Id, msg.Type)
		// Decode params from JSON payload (controller sends json.RawMessage)
		var params struct {
			Packages  []string `json:"packages"`
			Version   string   `json:"version"`
			OSVersion string   `json:"os_version"`
		}
		if len(msg.Params) > 0 {
			_ = json.Unmarshal(msg.Params, &params)
		}
		log.Printf("task %s params: packages=%v version=%s os=%s", msg.Id, params.Packages, params.Version, params.OSVersion)

		// Report running
		_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "running"})

		// Prepare package service with cache directory
		cacheDir := os.Getenv("VERTERA_CACHE_DIR")
		if cacheDir == "" { cacheDir = "/tmp/vertera/packages" }
		pkgSvc := packages.NewService(cacheDir)

		// For each requested package type, resolve download URLs, fetch required artifacts, and install
		var overallErr error
		for _, p := range params.Packages {
			pkgType := packages.PackageType(p)
			_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "running", Logs: "resolving package info: " + p})
			infos, err := pkgSvc.GetPackageInfo(pkgType, params.Version, params.OSVersion)
			if err != nil { overallErr = err; break }
			var paths []string
			for _, info := range infos {
				if !info.Required { continue }
				_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "running", Logs: "downloading: " + info.Name})
				path, err := pkgSvc.DownloadPackage(info)
				if err != nil { overallErr = err; break }
				_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "running", Logs: "downloaded: " + filepath.Base(path)})
				paths = append(paths, path)
			}
			if overallErr != nil { break }
			_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "running", Logs: "installing: " + p})
			instReq := packages.InstallRequest{PackageType: pkgType, Packages: paths, OSVersion: params.OSVersion}
			if err := pkgSvc.Install(instReq); err != nil { overallErr = err; break }
			_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "running", Logs: "installed: " + p})
		}

		if overallErr != nil {
			_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "failed", Error: overallErr.Error()})
			continue
		}
		// Report succeeded
		_, _ = cli.ReportTaskResult(ctx, &verterapb.TaskResult{Id: msg.Id, Status: "succeeded"})
	}
}
