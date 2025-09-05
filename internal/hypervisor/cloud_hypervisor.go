package hypervisor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ch "github.com/VerteraIO/cloud-hypervisor-go/chclient"
	"github.com/VerteraIO/cloud-hypervisor-go/unixhttp"
)

// CloudHypervisorClient wraps the generated Cloud Hypervisor client with Vertera-specific logic.
type CloudHypervisorClient struct {
	client *ch.ClientWithResponses
}

// NewCloudHypervisorClient creates a new Cloud Hypervisor client.
// For local development, it uses a Unix socket. For remote hosts, use NewCloudHypervisorClientHTTP.
func NewCloudHypervisorClient(socketPath string) (*CloudHypervisorClient, error) {
	httpc := unixhttp.NewClientWithTimeout(socketPath, 30*time.Second)
	
	client, err := ch.NewClientWithResponses("http://unix/api/v1", ch.WithHTTPClient(httpc))
	if err != nil {
		return nil, fmt.Errorf("failed to create CH client: %w", err)
	}

	return &CloudHypervisorClient{client: client}, nil
}

// NewCloudHypervisorClientHTTP creates a new Cloud Hypervisor client for HTTP endpoints.
func NewCloudHypervisorClientHTTP(endpoint string) (*CloudHypervisorClient, error) {
	client, err := ch.NewClientWithResponses(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create CH HTTP client: %w", err)
	}

	return &CloudHypervisorClient{client: client}, nil
}

// Ping checks if the Cloud Hypervisor VMM is responsive.
func (c *CloudHypervisorClient) Ping(ctx context.Context) (*ch.VmmPingResponse, error) {
	resp, err := c.client.GetVmmPingWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("ping failed with status %d", resp.StatusCode())
	}
	return resp.JSON200, nil
}

// CreateVM creates a new VM with the given configuration.
func (c *CloudHypervisorClient) CreateVM(ctx context.Context, config ch.VmConfig) error {
	resp, err := c.client.CreateVMWithResponse(ctx, config)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("create VM failed with status %d", resp.StatusCode())
	}
	return nil
}

// BootVM boots a previously created VM.
func (c *CloudHypervisorClient) BootVM(ctx context.Context) error {
	resp, err := c.client.BootVMWithResponse(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("boot VM failed with status %d", resp.StatusCode())
	}
	return nil
}

// ShutdownVM shuts down a running VM.
func (c *CloudHypervisorClient) ShutdownVM(ctx context.Context) error {
	resp, err := c.client.ShutdownVMWithResponse(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("shutdown VM failed with status %d", resp.StatusCode())
	}
	return nil
}

// GetVMInfo returns information about the current VM.
func (c *CloudHypervisorClient) GetVMInfo(ctx context.Context) (*ch.VmInfo, error) {
	resp, err := c.client.GetVmInfoWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get VM info failed with status %d", resp.StatusCode())
	}
	return resp.JSON200, nil
}
