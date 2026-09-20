package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"brain-master/backend/internal/queue"
	"brain-master/backend/internal/worker"
)

type APIHandler struct {
	Queue  *queue.JobManager
	Worker *worker.Client // nil => worker Python no disponible (modo simulación)
	startedAt time.Time
}

func NewAPIHandler(q *queue.JobManager) *APIHandler {
	return &APIHandler{Queue: q, startedAt: time.Now()}
}

// EnableCORS middleware
func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "healthy",
		"service":         "brain-master-gateway",
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"uptime_seconds":  int(time.Since(h.startedAt).Seconds()),
		"worker_connected": h.Worker != nil,
	})
}

func (h *APIHandler) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Mode           string `json:"mode"`
		Prompt         string `json:"prompt"`
		NegativePrompt string `json:"negative_prompt"`
		Model          string `json:"model"`
		Width          int    `json:"width"`
		Height         int    `json:"height"`
		Frames         int    `json:"frames"`
		Fps            int    `json:"fps"`
		Seed           int64  `json:"seed"`
		Steps          int    `json:"steps"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "El prompt es requerido", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		req.Model = "Wan2.1-T2V-14B"
	}
	if req.Width == 0 {
		req.Width = 1280
	}
	if req.Height == 0 {
		req.Height = 720
	}
	if req.Frames == 0 && req.Fps > 0 {
		req.Frames = 5 * req.Fps
	}
	// Cota de seguridad: un job no puede pedir marcos infinitos.
	if req.Frames > 576 {
		req.Frames = 576
	}
	// Cota de pasos de difusión: coherente con el clamp del worker (1..50).
	if req.Steps <= 0 {
		req.Steps = 20
	}
	if req.Steps > 50 {
		req.Steps = 50
	}

	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	job := &queue.GenerationJob{
		ID:             jobID,
		Mode:           req.Mode,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Model:          req.Model,
		Width:          req.Width,
		Height:         req.Height,
		Frames:         req.Frames,
		Fps:            req.Fps,
		Seed:           req.Seed,
		Steps:          req.Steps,
	}

	if err := h.Queue.SubmitJob(job); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusAccepted, job)
}

func (h *APIHandler) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Queue.ListJobs())
}

func (h *APIHandler) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Parámetro id requerido", http.StatusBadRequest)
		return
	}
	job, exists := h.Queue.GetJob(id)
	if !exists {
		http.Error(w, "Trabajo no encontrado", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// HandleCancelJob cancela un job en caliente (POST /api/v1/jobs/cancel?id=...).
func (h *APIHandler) HandleCancelJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Parámetro id requerido", http.StatusBadRequest)
		return
	}
	if err := h.Queue.CancelJob(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	job, _ := h.Queue.GetJob(id)
	writeJSON(w, http.StatusOK, job)
}

// HandleGpuTelemetry expone la telemetría de GPU del worker Python
// (GET /api/v1/gpu/telemetry). 503 si el worker no está conectado.
func (h *APIHandler) HandleGpuTelemetry(w http.ResponseWriter, r *http.Request) {
	if h.Worker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "worker Python no conectado; ejecutando en modo simulación",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	tel, err := h.Worker.GetGpuTelemetry(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_name":      tel.GetDeviceName(),
		"total_vram_mb":    tel.GetTotalVramMb(),
		"used_vram_mb":     tel.GetUsedVramMb(),
		"free_vram_mb":     tel.GetFreeVramMb(),
		"gpu_utilization":  tel.GetGpuUtilization(),
		"temperature_c":    tel.GetTemperatureC(),
	})
}

// HandleListModels expone el catálogo dinámico de modelos del worker
// (GET /api/v1/models). 503 si el worker no está conectado.
func (h *APIHandler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	if h.Worker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "worker Python no conectado",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	resp, err := h.Worker.ListModels(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	models := make([]map[string]interface{}, 0, len(resp.GetModels()))
	for _, m := range resp.GetModels() {
		models = append(models, map[string]interface{}{
			"id":        m.GetKey(),
			"name":      m.GetLabel(),
			"mode":      m.GetMode(),
			"engine":    m.GetEngine(),
			"pipeline":  m.GetPipeline(),
			"repo":      m.GetRepo(),
			"steps":     m.GetSteps(),
			"vram_gb":   m.GetVramGb(),
			"family":    m.GetFamily(),
			"available": m.GetAvailable(),
			"notes":     m.GetNotes(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models":       models,
		"device_name":  resp.GetDeviceName(),
		"cuda_available": resp.GetCudaAvailable(),
	})
}

// HandleEnhancePrompt delega la mejora de prompts al LLM del worker
// (POST /api/v1/prompts/enhance).
func (h *APIHandler) HandleEnhancePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	if h.Worker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "worker Python no conectado",
		})
		return
	}
	var req struct {
		RawPrompt   string `json:"raw_prompt"`
		TargetModel string `json:"target_model"`
		Style       string `json:"style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RawPrompt == "" {
		http.Error(w, "raw_prompt es requerido", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	enhanced, rationale, err := h.Worker.EnhancePrompt(ctx, req.RawPrompt, req.TargetModel, req.Style)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"enhanced_prompt": enhanced,
		"rationale":       rationale,
	})
}
