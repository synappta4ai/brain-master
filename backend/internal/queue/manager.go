package queue

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	inferencev1 "brain-master/backend/internal/proto"
)

// WorkerClient es la conexión gRPC hacia el motor de inferencia Python.
type WorkerClient interface {
	StreamGeneration(ctx context.Context, req *inferencev1.MediaGenerationRequest, onUpdate func(pct float64, status string, output string, errMsg string)) error
	Cancel(ctx context.Context, jobID string) error
	EnhancePrompt(ctx context.Context, rawPrompt, targetModel, style string) (string, string, error)
	GetGpuTelemetry(ctx context.Context) (*inferencev1.GpuTelemetryResponse, error)
	Close()
}

type JobStatus string

const (
	StatusQueued         JobStatus = "QUEUED"
	StatusProcessing     JobStatus = "PROCESSING"
	StatusCompleted      JobStatus = "COMPLETED"
	StatusFailed         JobStatus = "FAILED"
	StatusCancelled      JobStatus = "CANCELLED"
)

type GenerationJob struct {
	ID             string    `json:"id"`
	Mode           string    `json:"mode"` // "video" / "image"
	Prompt         string    `json:"prompt"`
	NegativePrompt string    `json:"negative_prompt,omitempty"`
	Model          string    `json:"model"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	Frames         int       `json:"frames"`
	Fps            int       `json:"fps,omitempty"`
	Seed           int64     `json:"seed,omitempty"`
	Steps          int       `json:"steps,omitempty"` // pasos de difusión (diffusers)
	Status         JobStatus `json:"status"`
	Step           int64     `json:"-"` // orden de persistencia
	Progress       float64   `json:"progress"`
	OutputPath     string    `json:"output_path,omitempty"`
	ArtifactURL    string    `json:"artifact_url,omitempty"` // URL pública (worker remoto)
	CreatedAt      time.Time `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// PersistLayer abstrae la persistencia de jobs (SQLite en producción).
type PersistLayer interface {
	UpsertJob(job *GenerationJob) error
	LoadAll() ([]*GenerationJob, error)
}

type JobManager struct {
	jobs      map[string]*GenerationJob
	jobQueue  chan *GenerationJob
	mu        sync.RWMutex
	onUpdate  func(job *GenerationJob)
	worker    WorkerClient // nil => modo simulación
	persist   PersistLayer // nil => solo memoria
	cancels   map[string]context.CancelFunc
	step      *int64 // persistencia secuencial
	// ArtifactBase: URL pública del artifact server del worker
	// (ej. https://tunel.trycloudflare.com). Cuando no está vacía, los jobs
	// COMPLETED exponen ArtifactURL para descargar desde el front.
	artifactBase string
}

// NewJobManager crea el manager. La variable BM_WORKER_ARTIFACT_BASE define
// la URL pública del artifact server (solo necesaria en despliegues
// divididos, donde el worker corre en otra máquina).

func NewJobManager(bufferSize int, onUpdate func(job *GenerationJob)) *JobManager {
	jm := &JobManager{
		jobs:         make(map[string]*GenerationJob),
		jobQueue:     make(chan *GenerationJob, bufferSize),
		onUpdate:     onUpdate,
		cancels:      make(map[string]context.CancelFunc),
		artifactBase: strings.TrimRight(os.Getenv("BM_WORKER_ARTIFACT_BASE"), "/"),
	}
	go jm.workerLoop()
	return jm
}

// AttachWorker conecta el cliente gRPC del worker Python. Cuando está seteado,
// las tareas se procesan vía streaming real en vez de simulación local.
func (jm *JobManager) AttachWorker(w WorkerClient) {
	jm.mu.Lock()
	jm.worker = w
	jm.mu.Unlock()
	log.Println("[Queue] Worker Python (gRPC) conectado: generación real habilitada")
}

// AttachPersist habilita persistencia SQLite.
func (jm *JobManager) AttachPersist(p PersistLayer) error {
	jm.mu.Lock()
	jm.persist = p
	jm.mu.Unlock()
	return jm.restore()
}

// restore recarga los jobs persistidos. Los que quedaron PROCESSING/QUEUED por
// un reinicio del gateway vuelven a QUEUED y se reencolan.
func (jm *JobManager) restore() error {
	jm.mu.RLock()
	p := jm.persist
	jm.mu.RUnlock()
	if p == nil {
		return nil
	}
	jobs, err := p.LoadAll()
	if err != nil {
		return err
	}
	jm.mu.Lock()
	var maxStep int64
	requeue := []*GenerationJob{}
	for _, j := range jobs {
		if j.Status == StatusProcessing || j.Status == StatusQueued {
			j.Status = StatusQueued
			j.Progress = 0
			requeue = append(requeue, j)
		}
		if j.Step > maxStep {
			maxStep = j.Step
		}
		jm.jobs[j.ID] = j
	}
	step := maxStep
	jm.step = &step
	jm.mu.Unlock()

	for _, j := range requeue {
		select {
		case jm.jobQueue <- j:
		default:
			log.Printf("[Queue] Cola llena al restaurar %s", j.ID)
		}
	}
	log.Printf("[Queue] Persistencia restaurada: %d jobs (%d reencolados)", len(jobs), len(requeue))
	return nil
}

// setStep devuelve el siguiente número de paso de persistencia.
func (jm *JobManager) nextStep() int64 {
	if jm.step == nil {
		s := int64(1)
		jm.step = &s
		return s
	}
	*jm.step++
	return *jm.step
}

func (jm *JobManager) SubmitJob(job *GenerationJob) error {
	jm.mu.Lock()
	job.Status = StatusQueued
	job.Progress = 0.0
	job.CreatedAt = time.Now()
	job.Step = jm.nextStep()
	jm.jobs[job.ID] = job
	p := jm.persist
	jm.mu.Unlock()
	if p != nil {
		p.UpsertJob(job)
	}

	if jm.onUpdate != nil {
		jm.onUpdate(job)
	}

	select {
	case jm.jobQueue <- job:
		log.Printf("[Queue] Trabajo encolado: %s\n", job.ID)
		return nil
	default:
		return fmt.Errorf("la cola de tareas de GPU está llena")
	}
}

func (jm *JobManager) GetJob(id string) (*GenerationJob, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	job, exists := jm.jobs[id]
	return job, exists
}

func (jm *JobManager) ListJobs() []*GenerationJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	list := make([]*GenerationJob, 0, len(jm.jobs))
	for _, j := range jm.jobs {
		list = append(list, j)
	}
	// Más recientes primero
	for i := 0; i < len(list); i++ {
		for k := i + 1; k < len(list); k++ {
			if list[k].CreatedAt.After(list[i].CreatedAt) {
				list[i], list[k] = list[k], list[i]
			}
		}
	}
	return list
}

// CancelJob cancela un job en caliente: si está en la cola se marca CANCELLED
// directo; si está procesándose se propaga al worker vía gRPC.
func (jm *JobManager) CancelJob(id string) error {
	jm.mu.Lock()
	job, ok := jm.jobs[id]
	if !ok {
		jm.mu.Unlock()
		return fmt.Errorf("job no encontrado")
	}
	worker := jm.worker
	persist := jm.persist

	switch job.Status {
	case StatusQueued:
		jm.setStatusLocked(job, StatusCancelled, "")
		jm.mu.Unlock()
		if persist != nil {
			persist.UpsertJob(job)
		}
		log.Printf("[Queue] Job %s cancelado antes de procesar", id)
		return nil
	case StatusProcessing:
		if cancel := jm.cancels[id]; cancel != nil {
			jm.mu.Unlock()
			cancel() // cierra el stream gRPC; el worker aborta el pipeline
			return nil
		}
		jm.mu.Unlock()
		if worker != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return worker.Cancel(ctx, id)
		}
		return fmt.Errorf("job sin contexto de cancelación activo")
	default:
		jm.mu.Unlock()
		return fmt.Errorf("el job está en estado %s y no puede cancelarse", job.Status)
	}
}

func (jm *JobManager) setStatusLocked(job *GenerationJob, s JobStatus, output string) {
	job.Status = s
	if output != "" {
		job.OutputPath = output
		// Despliegue dividido: el archivo vive en el worker; publicamos la
		// URL del artifact server para que el front la consuma directo.
		if jm.artifactBase != "" {
			if _, file := filepath.Split(output); file != "" {
				job.ArtifactURL = jm.artifactBase + "/artifacts/" + file
			}
		}
	}
	if s == StatusCompleted || s == StatusFailed || s == StatusCancelled {
		if job.CompletedAt == nil {
			now := time.Now()
			job.CompletedAt = &now
		}
		delete(jm.cancels, job.ID)
	}
}

// workerLoop procesa las tareas en orden secuencial hacia la GPU
func (jm *JobManager) workerLoop() {
	for job := range jm.jobQueue {
		// Un job cancelado mientras estaba encolado no se procesa.
		jm.mu.RLock()
		if job.Status == StatusCancelled {
			jm.mu.RUnlock()
			continue
		}
		worker := jm.worker
		persist := jm.persist
		jm.mu.RUnlock()

		// Contexto de cancelación: se registra EN el mismo bloque atómico que
		// marca PROCESSING, para que ningún CancelJob vea el estado sin poder
		// cancelar (ventana de carrera detectada por el test E2E).
		ctx, cancel := context.WithCancel(context.Background())
		jm.mu.Lock()
		job.Status = StatusProcessing
		jm.cancels[job.ID] = cancel
		jm.mu.Unlock()
		if jm.onUpdate != nil {
			jm.onUpdate(job)
		}
		if persist != nil {
			persist.UpsertJob(job)
		}

		log.Printf("[Queue] Iniciando procesamiento de Job %s (%s - %s)\n", job.ID, job.Mode, job.Model)

		if worker != nil {
			jm.runViaWorker(ctx, job, worker)
		} else {
			jm.runSimulation(ctx, job)
		}

		jm.mu.Lock()
		delete(jm.cancels, job.ID)
		jm.mu.Unlock()
		cancel()

		if persist != nil {
			persist.UpsertJob(job)
		}
		if jm.onUpdate != nil {
			jm.onUpdate(job)
		}
		log.Printf("[Queue] Job %s terminó con estado %s", job.ID, job.Status)
	}
}

// runViaWorker ejecuta la generación vía streaming gRPC contra el worker Python.
func (jm *JobManager) runViaWorker(ctx context.Context, job *GenerationJob, worker WorkerClient) {
	req := &inferencev1.MediaGenerationRequest{
		JobId:          job.ID,
		Mode:           job.Mode,
		ModelName:      job.Model,
		Prompt:         job.Prompt,
		NegativePrompt: job.NegativePrompt,
		Width:          int32(job.Width),
		Height:         int32(job.Height),
		NumFrames:      int32(job.Frames),
		Fps:            int32(job.Fps),
		Steps:          int32(job.Steps),
		Seed:           job.Seed,
	}

	err := worker.StreamGeneration(ctx, req, func(pct float64, status string, output string, errMsg string) {
		jm.mu.Lock()
		job.Progress = pct
		switch status {
		case "COMPLETED":
			jm.setStatusLocked(job, StatusCompleted, output)
		case "FAILED":
			jm.setStatusLocked(job, StatusFailed, "")
		case "CANCELLED":
			jm.setStatusLocked(job, StatusCancelled, "")
		}
		jm.mu.Unlock()
		if jm.onUpdate != nil {
			jm.onUpdate(job)
		}
	})

	jm.mu.Lock()
	defer jm.mu.Unlock()
	switch {
	case job.Status == StatusCompleted || job.Status == StatusFailed || job.Status == StatusCancelled:
		// El callback ya fijó el estado final
	case ctx.Err() != nil:
		jm.setStatusLocked(job, StatusCancelled, "")
		log.Printf("[Queue] Job %s cancelado", job.ID)
	case err != nil:
		jm.setStatusLocked(job, StatusFailed, "")
		log.Printf("[Queue] Error del worker en Job %s: %v", job.ID, err)
	default:
		jm.setStatusLocked(job, StatusCompleted, fmt.Sprintf("/outputs/%s.mp4", job.ID))
	}
}

// runSimulation es el fallback sin worker (la GPU/Python no están disponibles).
func (jm *JobManager) runSimulation(ctx context.Context, job *GenerationJob) {
	for step := 1; step <= 10; step++ {
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
			jm.mu.Lock()
			jm.setStatusLocked(job, StatusCancelled, "")
			jm.mu.Unlock()
			return
		}
		jm.mu.Lock()
		job.Progress = float64(step) * 10.0
		jm.mu.Unlock()

		if jm.onUpdate != nil {
			jm.onUpdate(job)
		}
	}

	jm.mu.Lock()
	jm.setStatusLocked(job, StatusCompleted, fmt.Sprintf("/outputs/%s.mp4", job.ID))
	jm.mu.Unlock()
}
