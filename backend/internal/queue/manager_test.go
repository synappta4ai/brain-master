package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	proto "brain-master/backend/internal/proto"
	"brain-master/backend/internal/queue"
	"brain-master/backend/internal/worker"
	"brain-master/backend/internal/wtest"
)

// waitFor hace polling hasta que cond() es verdadera o vence el timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condición no cumplida a tiempo")
}

// fakePersist implementa queue.PersistLayer en memoria.
type fakePersist struct {
	mu   sync.Mutex
	jobs map[string]*queue.GenerationJob
}

func newFakePersist(seed ...*queue.GenerationJob) *fakePersist {
	fp := &fakePersist{jobs: map[string]*queue.GenerationJob{}}
	for _, j := range seed {
		cp := *j
		fp.jobs[j.ID] = &cp
	}
	return fp
}

func (f *fakePersist) UpsertJob(job *queue.GenerationJob) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *job
	f.jobs[job.ID] = &cp
	return nil
}

func (f *fakePersist) LoadAll() ([]*queue.GenerationJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*queue.GenerationJob, 0, len(f.jobs))
	for _, j := range f.jobs {
		cp := *j
		out = append(out, &cp)
	}
	return out, nil
}

// stubWorker implementa queue.WorkerClient en memoria, sin gRPC.
type stubWorker struct {
	events      []evt // progreso intermedio
	finalStatus string
	finalOutput string
	streamErr   error // error de transporte del stream
	delay       time.Duration
	onStream    func(req *proto.MediaGenerationRequest)
	cancelled   chan string // job IDs cuya generación fue abortada por ctx
}

type evt struct {
	pct    float64
	status string
	output string
}

func (s *stubWorker) StreamGeneration(ctx context.Context, req *proto.MediaGenerationRequest, onUpdate func(pct float64, status string, output string, errMsg string)) error {
	if s.onStream != nil {
		s.onStream(req)
	}
	for _, e := range s.events {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			if s.cancelled != nil {
				s.cancelled <- req.GetJobId()
			}
			return ctx.Err()
		}
		onUpdate(e.pct, e.status, e.output, "")
	}
	if s.streamErr != nil {
		return s.streamErr
	}
	onUpdate(100, s.finalStatus, s.finalOutput, "")
	return nil
}

func (s *stubWorker) Cancel(ctx context.Context, jobID string) error { return nil }
func (s *stubWorker) EnhancePrompt(ctx context.Context, rawPrompt, targetModel, style string) (string, string, error) {
	return "enhanced", "stub", nil
}
func (s *stubWorker) GetGpuTelemetry(ctx context.Context) (*proto.GpuTelemetryResponse, error) {
	return &proto.GpuTelemetryResponse{DeviceName: "STUB-GPU"}, nil
}
func (s *stubWorker) Close() {}

// ---------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------

func TestSimulationCompletesJob(t *testing.T) {
	jm := queue.NewJobManager(8, nil)
	job := &queue.GenerationJob{ID: "t-sim", Mode: "video", Prompt: "test", Model: "mock"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 6*time.Second, func() bool {
		j, ok := jm.GetJob("t-sim")
		return ok && j.Status == queue.StatusCompleted && j.Progress == 100
	})
	j, _ := jm.GetJob("t-sim")
	if j.OutputPath == "" {
		t.Fatal("job completado sin OutputPath")
	}
}

func TestWorkerStreamCompletesJob(t *testing.T) {
	wc := &stubWorker{
		events:      []evt{{50, "GENERATING", ""}},
		finalStatus: "COMPLETED",
		finalOutput: "t-grpc.png",
	}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-grpc", Mode: "image", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, ok := jm.GetJob("t-grpc")
		return ok && j.Status == queue.StatusCompleted
	})
	j, _ := jm.GetJob("t-grpc")
	if j.OutputPath != "t-grpc.png" {
		t.Fatalf("OutputPath = %q, want t-grpc.png", j.OutputPath)
	}
	if j.CompletedAt == nil {
		t.Fatal("job completado sin CompletedAt")
	}
}

func TestArtifactURLWhenBaseConfigured(t *testing.T) {
	t.Setenv("BM_WORKER_ARTIFACT_BASE", "https://tunel.ejemplo.com")
	wc := &stubWorker{
		events:      []evt{},
		finalStatus: "COMPLETED",
		finalOutput: "/app/outputs/t-art.png",
	}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-art", Mode: "image", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, ok := jm.GetJob("t-art")
		return ok && j.Status == queue.StatusCompleted
	})
	j, _ := jm.GetJob("t-art")
	if j.ArtifactURL != "https://tunel.ejemplo.com/artifacts/t-art.png" {
		t.Fatalf("ArtifactURL = %q", j.ArtifactURL)
	}
	if j.CreatedAt.IsZero() {
		t.Fatal("CreatedAt debe seguir poblado")
	}
}

func TestNoArtifactURLWithoutBase(t *testing.T) {
	t.Setenv("BM_WORKER_ARTIFACT_BASE", "")
	wc := &stubWorker{
		events:      []evt{},
		finalStatus: "COMPLETED",
		finalOutput: "t-noart.png",
	}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-noart", Mode: "image", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, ok := jm.GetJob("t-noart")
		return ok && j.Status == queue.StatusCompleted
	})
	j, _ := jm.GetJob("t-noart")
	if j.ArtifactURL != "" {
		t.Fatalf("ArtifactURL = %q, want vacío", j.ArtifactURL)
	}
}

func TestWorkerErrorMarksJobFailed(t *testing.T) {
	wc := &stubWorker{streamErr: context.DeadlineExceeded}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-err", Mode: "video", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, _ := jm.GetJob("t-err")
		return j != nil && j.Status == queue.StatusFailed
	})
}

func TestWorkerFailedStatusPropagates(t *testing.T) {
	wc := &stubWorker{finalStatus: "FAILED", finalOutput: ""}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-failed", Mode: "video", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, _ := jm.GetJob("t-failed")
		return j != nil && j.Status == queue.StatusFailed
	})
}

func TestCancelProcessingJob(t *testing.T) {
	cancelled := make(chan string, 4)
	wc := &stubWorker{
		events:      []evt{{10, "GENERATING", ""}, {90, "GENERATING", ""}},
		finalStatus: "COMPLETED",
		finalOutput: "x.png",
		delay:       500 * time.Millisecond,
		cancelled:   cancelled,
	}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-cancel", Mode: "video", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	// Esperar a que esté en PROCESSING y luego cancelar en caliente.
	waitFor(t, 3*time.Second, func() bool {
		j, _ := jm.GetJob("t-cancel")
		return j != nil && j.Status == queue.StatusProcessing
	})
	if err := jm.CancelJob("t-cancel"); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, _ := jm.GetJob("t-cancel")
		return j != nil && j.Status == queue.StatusCancelled
	})
	select {
	case id := <-cancelled:
		if id != "t-cancel" {
			t.Fatalf("cancelación llegó al job equivocado: %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el stub nunca observó la cancelación por contexto")
	}
}

func TestCancelQueuedJob(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	wc := &stubWorker{
		events:      []evt{{50, "GENERATING", ""}},
		finalStatus: "COMPLETED",
		finalOutput: "a.png",
		onStream: func(req *proto.MediaGenerationRequest) {
			if req.GetJobId() == "j1" {
				<-release
				once.Do(func() {})
			}
		},
	}
	jm := queue.NewJobManager(8, nil)
	jm.AttachWorker(wc)

	j1 := &queue.GenerationJob{ID: "j1", Mode: "video", Prompt: "block", Model: "fake"}
	j2 := &queue.GenerationJob{ID: "j2", Mode: "video", Prompt: "queued", Model: "fake"}
	if err := jm.SubmitJob(j1); err != nil {
		t.Fatalf("SubmitJob j1: %v", err)
	}
	if err := jm.SubmitJob(j2); err != nil {
		t.Fatalf("SubmitJob j2: %v", err)
	}
	// j1 bloquea el worker loop => j2 permanece QUEUED.
	if err := jm.CancelJob("j2"); err != nil {
		t.Fatalf("CancelJob j2: %v", err)
	}
	j2b, _ := jm.GetJob("j2")
	if j2b.Status != queue.StatusCancelled {
		t.Fatalf("j2 status = %s, want CANCELLED", j2b.Status)
	}
	// Liberar j1 y verificar que el loop lo completa.
	close(release)
	waitFor(t, 3*time.Second, func() bool {
		j, _ := jm.GetJob("j1")
		return j != nil && j.Status == queue.StatusCompleted
	})
	// El job cancelado en cola no debe haberse procesado después.
	j2c, _ := jm.GetJob("j2")
	if j2c.Status != queue.StatusCancelled {
		t.Fatalf("j2 cambió de estado tras procesar j1: %s", j2c.Status)
	}
}

func TestCancelUnknownJobErrors(t *testing.T) {
	jm := queue.NewJobManager(8, nil)
	if err := jm.CancelJob("no-existe"); err == nil {
		t.Fatal("CancelJob de job inexistente debería fallar")
	}
}

func TestCancelTerminalJobErrors(t *testing.T) {
	jm := queue.NewJobManager(8, nil)
	job := &queue.GenerationJob{ID: "t-term", Mode: "video", Prompt: "test", Model: "mock"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 6*time.Second, func() bool {
		j, _ := jm.GetJob("t-term")
		return j != nil && j.Status == queue.StatusCompleted
	})
	if err := jm.CancelJob("t-term"); err == nil {
		t.Fatal("cancelar un job COMPLETED debería fallar")
	}
}

func TestAttachPersistRestoresAndRequeues(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	seed := []*queue.GenerationJob{
		{ID: "hist", Mode: "video", Prompt: "done", Model: "m", Status: queue.StatusCompleted, Progress: 100, CreatedAt: old, Step: 1},
		{ID: "orph", Mode: "video", Prompt: "orphan", Model: "m", Status: queue.StatusProcessing, Progress: 40, CreatedAt: old, Step: 2},
	}
	fp := newFakePersist(seed...)
	jm := queue.NewJobManager(8, nil)
	if err := jm.AttachPersist(fp); err != nil {
		t.Fatalf("AttachPersist: %v", err)
	}
	// El huérfano debe reencolarse y completarse (sin worker => simulación).
	waitFor(t, 6*time.Second, func() bool {
		j, ok := jm.GetJob("orph")
		return ok && j.Status == queue.StatusCompleted
	})
	// El completado histórico permanece intacto.
	j, _ := jm.GetJob("hist")
	if j.Status != queue.StatusCompleted || j.Progress != 100 {
		t.Fatalf("hist = %s/%v, want COMPLETED/100", j.Status, j.Progress)
	}
	// El contador de pasos continúa tras la restauración.
	nuevo := &queue.GenerationJob{ID: "new", Mode: "video", Prompt: "n", Model: "m"}
	if err := jm.SubmitJob(nuevo); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	if nuevo.Step <= 2 {
		t.Fatalf("Step = %d, debe continuar desde el máximo restaurado (>2)", nuevo.Step)
	}
}

func TestPersistUpsertsDuringLifecycle(t *testing.T) {
	fp := newFakePersist()
	wc := &stubWorker{events: []evt{{40, "GENERATING", ""}}, finalStatus: "COMPLETED", finalOutput: "p.png"}
	jm := queue.NewJobManager(8, nil)
	jm.AttachPersist(fp)
	jm.AttachWorker(wc)

	job := &queue.GenerationJob{ID: "t-persist", Mode: "image", Prompt: "test", Model: "fake"}
	if err := jm.SubmitJob(job); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		j, _ := jm.GetJob("t-persist")
		return j != nil && j.Status == queue.StatusCompleted
	})
	waitFor(t, 2*time.Second, func() bool {
		fp.mu.Lock()
		defer fp.mu.Unlock()
		pj, ok := fp.jobs["t-persist"]
		return ok && pj.Status == queue.StatusCompleted && pj.OutputPath == "p.png"
	})
}

// Verifica que el cliente gRPC real (worker.Client) funciona contra un
// servidor falso y satisface la interfaz queue.WorkerClient.
func TestRealGRPCClientAgainstFakeServer(t *testing.T) {
	svc := &wtest.FakeInferenceService{OutputDir: t.TempDir(), StepDelay: 2 * time.Millisecond}
	addr, stop := wtest.StartFakeWorker(t, svc)
	defer stop()

	wc, err := worker.NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer wc.Close()

	var _ queue.WorkerClient = wc // compilación: satisface la interfaz

	var got []*proto.GenerationProgressResponse
	err = wc.StreamGeneration(context.Background(), &proto.MediaGenerationRequest{
		JobId: "g1", Mode: "image", ModelName: "fake", Prompt: "p", Steps: 3,
	}, func(pct float64, status, output, errMsg string) {
		got = append(got, &proto.GenerationProgressResponse{Percentage: float32(pct), Status: status, OutputFilePath: output})
	})
	if err != nil {
		t.Fatalf("StreamGeneration: %v", err)
	}
	if len(got) == 0 || got[len(got)-1].Status != "COMPLETED" || got[len(got)-1].OutputFilePath != "g1.png" {
		t.Fatalf("stream inesperado: %+v", got)
	}

	tel, err := wc.GetGpuTelemetry(context.Background())
	if err != nil || tel.GetDeviceName() != "FAKE-GPU" {
		t.Fatalf("GetGpuTelemetry = %v, %v", tel, err)
	}
	enh, rat, err := wc.EnhancePrompt(context.Background(), "hola", "wan", "cinematic")
	if err != nil || enh != "enhanced: hola" || rat == "" {
		t.Fatalf("EnhancePrompt = %q, %q, %v", enh, rat, err)
	}
	// Cancelar un job que ya terminó debe fallar (el worker real también
	// responde accepted=false cuando no tiene el job activo).
	if err := wc.Cancel(context.Background(), "g1"); err == nil {
		t.Fatal("Cancel de job inactivo debería fallar")
	}
}
