// Package worker implementa el cliente gRPC del gateway hacia el
// motor de inferencia Python (capa 3 de la arquitectura híbrida).
package worker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	inferencev1 "brain-master/backend/internal/proto"
	"brain-master/backend/internal/queue"
)

// Client implementa queue.WorkerClient sobre gRPC.
type Client struct {
	conn   *grpc.ClientConn
	api    inferencev1.InferenceServiceClient
	cancel sync.Map // job_id -> context.CancelFunc del stream activo
}

// NewClient conecta al worker Python.
//
// Formatos de dirección:
//   - host:puerto           → gRPC plano (local/docker: 127.0.0.1:50051)
//   - tls://host:puerto     → gRPC sobre TLS (túneles cloudflared de
//     Colab/Kaggle, o worker remoto con certificado público)
func NewClient(addr string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	credsOpt := grpc.WithTransportCredentials(insecure.NewCredentials())
	if rest, ok := strings.CutPrefix(addr, "tls://"); ok {
		addr = rest
		credsOpt = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	}
	conn, err := grpc.NewClient(addr, credsOpt)
	if err != nil {
		return nil, fmt.Errorf("conectando al worker %s: %w", addr, err)
	}
	// Ping inicial: si el worker no responde, fallamos rápido.
	pingCtx, pingCancel := context.WithTimeout(ctx, 4*time.Second)
	defer pingCancel()
	probe := inferencev1.NewInferenceServiceClient(conn)
	if _, err := probe.GetGpuTelemetry(pingCtx, &inferencev1.GpuTelemetryRequest{}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("worker %s no responde: %w", addr, err)
	}
	log.Printf("[Worker] Conectado a Python en %s", addr)
	return &Client{conn: conn, api: probe}, nil
}

// StreamGeneration ejecuta GenerateMedia y delega cada evento del stream al
// callback onUpdate (progreso, estado, output, error).
func (c *Client) StreamGeneration(ctx context.Context, req *inferencev1.MediaGenerationRequest, onUpdate func(pct float64, status string, output string, errMsg string)) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.cancel.Store(req.JobId, cancel)
	defer c.cancel.Delete(req.JobId)

	stream, err := c.api.GenerateMedia(streamCtx, req)
	if err != nil {
		return fmt.Errorf("abriendo stream: %w", err)
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Un cancel del contexto llega aquí; el queue manager distingue el caso.
			return fmt.Errorf("stream interrumpido: %w", err)
		}
		onUpdate(float64(resp.Percentage), resp.Status, resp.OutputFilePath, resp.ErrorMessage)
		if resp.Status == "COMPLETED" || resp.Status == "FAILED" || resp.Status == "CANCELLED" {
			return nil
		}
	}
}

// Cancel solicita al worker abortar una generación en curso.
func (c *Client) Cancel(ctx context.Context, jobID string) error {
	if cancel, ok := c.cancel.Load(jobID); ok {
		cancel.(context.CancelFunc)()
		return nil
	}
	resp, err := c.api.CancelGeneration(ctx, &inferencev1.CancelRequest{JobId: jobID})
	if err != nil {
		return fmt.Errorf("cancel %s: %w", jobID, err)
	}
	if !resp.Accepted {
		return fmt.Errorf("el worker no tiene activo el job %s", jobID)
	}
	return nil
}

// EnhancePrompt pide al LLM local del worker una versión mejorada del prompt.
func (c *Client) EnhancePrompt(ctx context.Context, rawPrompt, targetModel, style string) (string, string, error) {
	resp, err := c.api.EnhancePrompt(ctx, &inferencev1.PromptEnhanceRequest{
		RawPrompt:   rawPrompt,
		TargetModel: targetModel,
		Style:       style,
	})
	if err != nil {
		return "", "", fmt.Errorf("enhance: %w", err)
	}
	return resp.EnhancedPrompt, resp.Rationale, nil
}

// GetGpuTelemetry consulta el estado de la GPU del worker.
func (c *Client) GetGpuTelemetry(ctx context.Context) (*inferencev1.GpuTelemetryResponse, error) {
	return c.api.GetGpuTelemetry(ctx, &inferencev1.GpuTelemetryRequest{})
}

// ListModels obtiene el catálogo dinámico de modelos del worker.
func (c *Client) ListModels(ctx context.Context) (*inferencev1.ListModelsResponse, error) {
	return c.api.ListModels(ctx, &inferencev1.ListModelsRequest{})
}

// Close libera la conexión gRPC.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// Compile-time check: Client satisface queue.WorkerClient.
var _ queue.WorkerClient = (*Client)(nil)
