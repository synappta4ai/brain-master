package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"brain-master/backend/internal/api"
	"brain-master/backend/internal/queue"
	"brain-master/backend/internal/store"
	"brain-master/backend/internal/worker"
	"brain-master/backend/internal/ws"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" || port == "0" {
		port = "8080"
	}
	workerAddr := os.Getenv("PYTHON_WORKER_HOST")
	if workerAddr == "" {
		workerAddr = "127.0.0.1:50051"
	}
	dbPath := os.Getenv("BM_DB_PATH")
	if dbPath == "" {
		dbPath = "brain-master.db"
	}

	// 1. Inicializar WebSocket Hub
	wsHub := ws.NewHub()
	go wsHub.Run()

	// 2. Persistencia SQLite: los jobs sobreviven a reinicios del gateway.
	persist, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		log.Printf("⚠️  [Main] Sin persistencia: %v", err)
		persist = nil
	}

	// 3. Inicializar Queue Manager conectado al WebSocket Hub
	queueManager := queue.NewJobManager(50, func(job *queue.GenerationJob) {
		// Emitir actualización de progreso a todos los clientes Angular conectados
		wsHub.BroadcastJSON(map[string]interface{}{
			"type": "JOB_UPDATE",
			"data": job,
		})
	})
	if persist != nil {
		if err := queueManager.AttachPersist(persist); err != nil {
			log.Printf("⚠️  [Main] No se pudo restaurar el historial: %v", err)
		}
	}

	// 4. Conectar al worker Python (capa 3) vía gRPC. Si no está disponible
	// al arrancar, un goroutine reintenta cada 10s hasta conectarlo (mientras
	// tanto el gateway opera en modo simulación).
	apiHandler := api.NewAPIHandler(queueManager)
	connectWorker := func(wc *worker.Client) {
		queueManager.AttachWorker(wc)
		apiHandler.Worker = wc
		log.Printf("🧠 [Main] Motor de inferencia Python en %s (gRPC streaming real)", workerAddr)
	}
	if wc, err := worker.NewClient(workerAddr); err != nil {
		log.Printf("⚠️  [Main] Worker Python no disponible aún (%v): reintentando...", err)
		go func() {
			for {
				time.Sleep(10 * time.Second)
				wc, err := worker.NewClient(workerAddr)
				if err == nil {
					connectWorker(wc)
					return
				}
				log.Printf("[Main] Reintento de conexión al worker falló: %v", err)
			}
		}()
	} else {
		defer wc.Close()
		connectWorker(wc)
	}

	_ = os.MkdirAll("./outputs", 0o755)

	mux := http.NewServeMux()

	// Rutas de API REST
	mux.HandleFunc("/api/v1/health", apiHandler.HandleHealth)
	mux.HandleFunc("/api/v1/jobs/create", apiHandler.HandleCreateJob)
	mux.HandleFunc("/api/v1/jobs", apiHandler.HandleListJobs)
	mux.HandleFunc("/api/v1/jobs/detail", apiHandler.HandleGetJob)
	mux.HandleFunc("/api/v1/jobs/cancel", apiHandler.HandleCancelJob)
	mux.HandleFunc("/api/v1/gpu/telemetry", apiHandler.HandleGpuTelemetry)
	mux.HandleFunc("/api/v1/models", apiHandler.HandleListModels)
	mux.HandleFunc("/api/v1/prompts/enhance", apiHandler.HandleEnhancePrompt)

	// Ruta WebSocket para Angular
	mux.HandleFunc("/ws/telemetry", wsHub.HandleWebSocket)

	// Servir archivos estáticos/outputs
	mux.Handle("/outputs/", http.StripPrefix("/outputs/", http.FileServer(http.Dir("./outputs"))))

	handlerWithCORS := api.EnableCORS(mux)

	mode := "SIMULACIÓN"
	if apiHandler.Worker != nil {
		mode = "WORKER PYTHON REAL"
	}
	log.Printf("🚀 [Brain-Master] Orquestador y API Gateway en Go iniciado en :%s (modo %s)", port, mode)
	log.Printf("📡 WebSocket Hub activo en ws://127.0.0.1:%s/ws/telemetry", port)
	log.Printf("🩺 Health check: http://127.0.0.1:%s/api/v1/health", port)

	if err := http.ListenAndServe(":"+port, handlerWithCORS); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
