// Package wtest provee un servidor gRPC falso que imita al worker Python
// de inferencia, para tests unitarios y de integración del gateway Go.
package wtest

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	inferencev1 "brain-master/backend/internal/proto"
)

// FakeInferenceService implementa inferencev1.InferenceServiceServer con
// comportamiento determinista y configurable por test.
type FakeInferenceService struct {
	inferencev1.UnimplementedInferenceServiceServer

	// Steps: cantidad de eventos GENERATING antes del estado final (0 => 3).
	Steps int
	// StepDelay: pausa entre eventos (0 => 10ms).
	StepDelay time.Duration
	// OutputDir: dónde escribir el artefacto final.
	OutputDir string
	// FailWithError: si no está vacío, el stream termina en FAILED.
	FailWithError string
	// OnGenerate: override total del comportamiento (opcional).
	OnGenerate func(req *inferencev1.MediaGenerationRequest, stream grpc.ServerStreamingServer[inferencev1.GenerationProgressResponse]) error

	cancelled      atomic.Int32
	cancelRequests sync.Map // job_id -> chan struct{} para despertar CancelGeneration
}

// CancelCount devuelve cuántas generaciones fueron abortadas por cancelación.
func (f *FakeInferenceService) CancelCount() int32 { return f.cancelled.Load() }

func (f *FakeInferenceService) GenerateMedia(req *inferencev1.MediaGenerationRequest, stream grpc.ServerStreamingServer[inferencev1.GenerationProgressResponse]) error {
	if f.OnGenerate != nil {
		return f.OnGenerate(req, stream)
	}
	steps := f.Steps
	if steps == 0 {
		steps = 3
	}
	delay := f.StepDelay
	if delay == 0 {
		delay = 10 * time.Millisecond
	}

	send := func(status string, pct float64, out, errMsg string) error {
		return stream.Send(&inferencev1.GenerationProgressResponse{
			JobId: req.GetJobId(), CurrentStep: int32(steps), TotalSteps: int32(steps),
			Percentage: float32(pct), Status: status, OutputFilePath: out, ErrorMessage: errMsg,
		})
	}

	for i := 1; i <= steps; i++ {
		select {
		case <-time.After(delay):
		case <-stream.Context().Done():
			f.cancelled.Add(1)
			return stream.Context().Err()
		}
		if err := send("GENERATING", float64(i)/float64(steps)*100, "", ""); err != nil {
			return err
		}
	}
	if f.FailWithError != "" {
		return send("FAILED", 0, "", f.FailWithError)
	}
	outName := req.GetJobId() + ".png"
	if err := os.WriteFile(filepath.Join(f.OutputDir, outName), []byte("fake-artifact"), 0o644); err != nil {
		return send("FAILED", 0, "", fmt.Sprintf("escribiendo output: %v", err))
	}
	return send("COMPLETED", 100, outName, "")
}

// CancelGeneration marca la cancelación del job activo y espera a que
// GenerateMedia confirme, imitando el contrato del worker Python real.
func (f *FakeInferenceService) CancelGeneration(ctx context.Context, req *inferencev1.CancelRequest) (*inferencev1.CancelResponse, error) {
	deadline := time.After(2 * time.Second)
	for f.cancelled.Load() == 0 {
		select {
		case <-deadline:
			return &inferencev1.CancelResponse{JobId: req.GetJobId(), Accepted: false}, nil
		case <-ctx.Done():
			return &inferencev1.CancelResponse{JobId: req.GetJobId(), Accepted: false}, nil
		case <-time.After(5 * time.Millisecond):
		}
	}
	return &inferencev1.CancelResponse{JobId: req.GetJobId(), Accepted: true}, nil
}

func (f *FakeInferenceService) EnhancePrompt(ctx context.Context, req *inferencev1.PromptEnhanceRequest) (*inferencev1.PromptEnhanceResponse, error) {
	return &inferencev1.PromptEnhanceResponse{
		EnhancedPrompt: "enhanced: " + req.GetRawPrompt(),
		Rationale:      "fake rationale",
	}, nil
}

func (f *FakeInferenceService) GetGpuTelemetry(ctx context.Context, req *inferencev1.GpuTelemetryRequest) (*inferencev1.GpuTelemetryResponse, error) {
	return &inferencev1.GpuTelemetryResponse{
		DeviceName: "FAKE-GPU", TotalVramMb: 24576, UsedVramMb: 2048,
		FreeVramMb: 22528, GpuUtilization: 12.5, TemperatureC: 61.0,
	}, nil
}

func (f *FakeInferenceService) ListModels(ctx context.Context, req *inferencev1.ListModelsRequest) (*inferencev1.ListModelsResponse, error) {
	resp := &inferencev1.ListModelsResponse{DeviceName: "FAKE-GPU", CudaAvailable: true}
	for _, m := range []struct{ key, mode, engine string }{
		{"SD-Tiny-Test", "image", "diffusers"},
		{"Wan2.1-T2V-1.3B", "video", "diffusers"},
	} {
		resp.Models = append(resp.Models, &inferencev1.ModelInfo{
			Key: m.key, Label: m.key, Mode: m.mode, Engine: m.engine,
			Available: true, Steps: 20, VramGb: 4,
		})
	}
	return resp, nil
}

// StartFakeWorker levanta el servidor gRPC falso en un puerto efímero y
// devuelve (dirección, stop). stop es seguro de llamar varias veces.
func StartFakeWorker(t testing.TB, svc *FakeInferenceService) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake worker: %v", err)
	}
	srv := grpc.NewServer()
	inferencev1.RegisterInferenceServiceServer(srv, svc)
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.Serve(lis)
	}()
	stopped := false
	return lis.Addr().String(), func() {
		if !stopped {
			stopped = true
			srv.Stop()
			<-done
		}
	}
}
