// Test de integración end-to-end del gateway: reproduce el cableado de
// main.go (API REST + worker gRPC falso + SQLite temporal) y valida el
// ciclo completo create → processing → completed → output, más cancelación,
// telemetría, enhance y CORS.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"brain-master/backend/internal/api"
	"brain-master/backend/internal/queue"
	"brain-master/backend/internal/store"
	"brain-master/backend/internal/wtest"
	"brain-master/backend/internal/worker"
	"brain-master/backend/internal/ws"
)

type e2eEnv struct {
	server *httptest.Server
	queue  *queue.JobManager
	stop   func()
}

func newE2EEnv(t *testing.T, withWorker bool) *e2eEnv {
	t.Helper()
	return newE2EEnvDelay(t, withWorker, 15*time.Millisecond)
}

func newE2EEnvDelay(t *testing.T, withWorker bool, stepDelay time.Duration) *e2eEnv {
	t.Helper()

	var stopFns []func()
	stop := func() {
		for i := len(stopFns) - 1; i >= 0; i-- {
			stopFns[i]()
		}
	}

	persist, err := store.NewSQLiteStore(t.TempDir() + "/e2e.db")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	stopFns = append(stopFns, func() { persist.Close() })

	wsHub := ws.NewHub()
	go wsHub.Run()

	qm := queue.NewJobManager(16, func(job *queue.GenerationJob) {
		wsHub.BroadcastJSON(map[string]any{"type": "JOB_UPDATE", "data": job})
	})
	if err := qm.AttachPersist(persist); err != nil {
		t.Fatalf("AttachPersist: %v", err)
	}

	apiHandler := api.NewAPIHandler(qm)
	if withWorker {
		svc := &wtest.FakeInferenceService{OutputDir: t.TempDir(), StepDelay: stepDelay}
		addr, stopWorker := wtest.StartFakeWorker(t, svc)
		stopFns = append(stopFns, stopWorker)
		wc, err := worker.NewClient(addr)
		if err != nil {
			t.Fatalf("worker.NewClient: %v", err)
		}
		stopFns = append(stopFns, wc.Close)
		qm.AttachWorker(wc)
		apiHandler.Worker = wc
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", apiHandler.HandleHealth)
	mux.HandleFunc("/api/v1/jobs/create", apiHandler.HandleCreateJob)
	mux.HandleFunc("/api/v1/jobs", apiHandler.HandleListJobs)
	mux.HandleFunc("/api/v1/jobs/detail", apiHandler.HandleGetJob)
	mux.HandleFunc("/api/v1/jobs/cancel", apiHandler.HandleCancelJob)
	mux.HandleFunc("/api/v1/gpu/telemetry", apiHandler.HandleGpuTelemetry)
	mux.HandleFunc("/api/v1/models", apiHandler.HandleListModels)
	mux.HandleFunc("/api/v1/prompts/enhance", apiHandler.HandleEnhancePrompt)

	srv := httptest.NewServer(api.EnableCORS(mux))
	t.Cleanup(srv.Close)
	t.Cleanup(stop)

	return &e2eEnv{server: srv, queue: qm, stop: stop}
}

func (e *e2eEnv) post(t *testing.T, path, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(e.server.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		// Los errores de validación van como texto plano (http.Error).
		return resp.StatusCode, map[string]any{"_raw": strings.TrimSpace(string(raw))}
	}
	return resp.StatusCode, out
}

func (e *e2eEnv) get(t *testing.T, path string) (int, any) {
	t.Helper()
	resp, err := http.Get(e.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var out any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decodificando respuesta de %s: %v", path, err)
	}
	return resp.StatusCode, out
}

func waitForStatus(t *testing.T, e *e2eEnv, id, status string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		code, out := e.get(t, "/api/v1/jobs/detail?id="+id)
		if code == http.StatusOK {
			last = out.(map[string]any)
			if last["status"] == status {
				return last
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("job %s no llegó a %s a tiempo (último: %v)", id, status, last)
	return nil
}

func TestE2EHealthWithWorker(t *testing.T) {
	env := newE2EEnv(t, true)
	code, out := env.get(t, "/api/v1/health")
	if code != http.StatusOK {
		t.Fatalf("health code = %d", code)
	}
	if out.(map[string]any)["worker_connected"] != true {
		t.Fatalf("worker_connected = %v, want true", out.(map[string]any)["worker_connected"])
	}
}

func TestE2ECreateAndCompleteJob(t *testing.T) {
	env := newE2EEnv(t, true)

	code, out := env.post(t, "/api/v1/jobs/create", `{
		"mode": "image", "prompt": "a mountain at dusk", "model": "Flux.1-dev",
		"width": 1024, "height": 1024
	}`)
	if code != http.StatusAccepted {
		t.Fatalf("create code = %d, body=%v", code, out)
	}
	id := out["id"].(string)
	if !strings.HasPrefix(id, "job_") {
		t.Fatalf("id inesperado: %s", id)
	}
	if out["status"] != string(queue.StatusQueued) && out["status"] != string(queue.StatusProcessing) {
		t.Fatalf("estado inicial = %v", out["status"])
	}

	done := waitForStatus(t, env, id, "COMPLETED")
	if done["progress"].(float64) != 100 {
		t.Fatalf("progress = %v, want 100", done["progress"])
	}
	output := done["output_path"].(string)
	if output != id+".png" {
		t.Fatalf("output_path = %s, want %s.png", output, id)
	}
	// El detalle individual coincide con el listado.
	_, detail := env.get(t, "/api/v1/jobs/detail?id="+id)
	if detail.(map[string]any)["id"] != id {
		t.Fatal("detail no coincide con el id creado")
	}
	_, list := env.get(t, "/api/v1/jobs")
	found := false
	for _, item := range list.([]any) {
		if item.(map[string]any)["id"] == id {
			found = true
		}
	}
	if !found {
		t.Fatal("el job no aparece en el listado")
	}
}

func TestE2ECreateValidations(t *testing.T) {
	env := newE2EEnv(t, false)

	// Prompt vacío => 400.
	if code, _ := env.post(t, "/api/v1/jobs/create", `{"mode":"image","prompt":""}`); code != http.StatusBadRequest {
		t.Fatalf("prompt vacío code = %d, want 400", code)
	}
	// JSON inválido => 400.
	code, _ := env.post(t, "/api/v1/jobs/create", `{invalid`)
	if code != http.StatusBadRequest {
		t.Fatalf("json inválido code = %d, want 400", code)
	}
	// Defaults: sin modelo ni dimensiones usa valores por defecto y recorta frames.
	code, out := env.post(t, "/api/v1/jobs/create", `{"prompt":"defaults","fps":999}`)
	if code != http.StatusAccepted {
		t.Fatalf("create con defaults code = %d", code)
	}
	if out["model"] != "Wan2.1-T2V-14B" {
		t.Fatalf("modelo por defecto = %v", out["model"])
	}
	if out["frames"].(float64) > 576 {
		t.Fatalf("frames fuera de cota = %v", out["frames"])
	}
}

func TestE2ECancelRunningJob(t *testing.T) {
	// Worker lento: el job sigue PROCESSING cuando llega la cancelación.
	env := newE2EEnvDelay(t, true, 600*time.Millisecond)

	_, out := env.post(t, "/api/v1/jobs/create", `{"prompt":"cancelable","model":"fake"}`)
	id := out["id"].(string)
	waitForStatus(t, env, id, "PROCESSING")

	code, cancelled := env.post(t, "/api/v1/jobs/cancel?id="+id, "")
	t.Logf("cancel → HTTP %d body=%v", code, cancelled)
	if code != http.StatusOK {
		t.Fatalf("cancel code = %d, body=%v", code, cancelled)
	}
	done := waitForStatus(t, env, id, "CANCELLED")
	if op := done["output_path"]; op != nil && op != "" {
		t.Fatalf("job cancelado con output: %v", op)
	}
	// Cancelar de nuevo (ya terminal) => 409.
	if code, _ := env.post(t, "/api/v1/jobs/cancel?id="+id, ""); code != http.StatusConflict {
		t.Fatalf("segunda cancelación code = %d, want 409", code)
	}
}

func TestE2ECancelUnknownJob(t *testing.T) {
	env := newE2EEnv(t, false)
	code, out := env.post(t, "/api/v1/jobs/cancel?id=ghost", "")
	if code != http.StatusConflict {
		t.Fatalf("cancel fantasma code = %d, body=%v", code, out)
	}
}

func TestE2EGpuTelemetry(t *testing.T) {
	env := newE2EEnv(t, true)
	code, out := env.get(t, "/api/v1/gpu/telemetry")
	if code != http.StatusOK {
		t.Fatalf("telemetry code = %d, body=%v", code, out)
	}
	m := out.(map[string]any)
	if m["device_name"] != "FAKE-GPU" || m["total_vram_mb"].(float64) != 24576 {
		t.Fatalf("telemetría inesperada: %v", m)
	}
}

func TestE2EListModels(t *testing.T) {
	env := newE2EEnv(t, true)
	code, out := env.get(t, "/api/v1/models")
	if code != http.StatusOK {
		t.Fatalf("models code = %d, body=%v", code, out)
	}
	m := out.(map[string]any)
	models := m["models"].([]any)
	if len(models) != 2 {
		t.Fatalf("models len = %d, want 2", len(models))
	}
	first := models[0].(map[string]any)
	if first["id"] != "SD-Tiny-Test" || first["available"] != true {
		t.Fatalf("modelo inesperado: %v", first)
	}
	if m["cuda_available"] != true {
		t.Fatalf("cuda_available = %v", m["cuda_available"])
	}
}

func TestE2EEnhancePrompt(t *testing.T) {
	env := newE2EEnv(t, true)
	code, out := env.post(t, "/api/v1/prompts/enhance", `{"raw_prompt":"a cat","target_model":"wan","style":"cinematic"}`)
	if code != http.StatusOK {
		t.Fatalf("enhance code = %d, body=%v", code, out)
	}
	if !strings.HasPrefix(out["enhanced_prompt"].(string), "enhanced: a cat") {
		t.Fatalf("enhanced_prompt inesperado: %v", out)
	}
}

func TestE2EJobsDetailNotFound(t *testing.T) {
	env := newE2EEnv(t, false)
	resp, err := http.Get(env.server.URL + "/api/v1/jobs/detail?id=missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("detail missing code = %d, want 404", resp.StatusCode)
	}
}

func TestE2ECorsHeaders(t *testing.T) {
	env := newE2EEnv(t, false)
	req, _ := http.NewRequest(http.MethodOptions, env.server.URL+"/api/v1/jobs", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preflight code = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("falta Access-Control-Allow-Origin en preflight")
	}
}

func TestE2EPersistenceAcrossRestart(t *testing.T) {
	// Primer ciclo: crear y completar un job con persistencia en disco.
	dbPath := t.TempDir() + "/restart.db"
	persist, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	qm := queue.NewJobManager(8, nil)
	if err := qm.AttachPersist(persist); err != nil {
		t.Fatal(err)
	}
	job := &queue.GenerationJob{ID: "job_restart", Mode: "video", Prompt: "restart", Model: "m"}
	if err := qm.SubmitJob(job); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if j, _ := qm.GetJob("job_restart"); j != nil && j.Status == queue.StatusCompleted {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	persist.Close()

	// Segundo ciclo: "reiniciar" el gateway sobre la misma base.
	persist2, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer persist2.Close()
	qm2 := queue.NewJobManager(8, nil)
	if err := qm2.AttachPersist(persist2); err != nil {
		t.Fatal(err)
	}
	j, ok := qm2.GetJob("job_restart")
	if !ok {
		t.Fatal("el job no sobrevivió al reinicio")
	}
	if j.Status != queue.StatusCompleted {
		t.Fatalf("estado tras reinicio = %s, want COMPLETED", j.Status)
	}
	if fmt.Sprint(j.ID) != "job_restart" {
		t.Fatal("id inconsistente tras reinicio")
	}
}
