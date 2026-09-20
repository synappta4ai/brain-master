import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface CreateJobPayload {
  mode: 'video' | 'image';
  prompt: string;
  negative_prompt?: string;
  model: string;
  width: number;
  height: number;
  frames: number;
  fps?: number;
  seed?: number;
  /** Pasos de inferencia (diffusers); el worker lo acota 1..50. */
  steps?: number;
}

export interface Job {
  id: string;
  mode: string;
  prompt: string;
  negative_prompt?: string;
  model: string;
  width: number;
  height: number;
  frames: number;
  fps?: number;
  seed?: number;
  status: 'QUEUED' | 'PROCESSING' | 'COMPLETED' | 'FAILED' | 'CANCELLED';
  progress: number;
  output_path?: string;
  /** URL pública del artifact server (worker remoto en la nube). */
  artifact_url?: string;
  created_at: string;
  completed_at?: string;
}

export interface GpuTelemetry {
  device_name: string;
  total_vram_mb: number;
  used_vram_mb: number;
  free_vram_mb: number;
  gpu_utilization: number;
  temperature_c: number;
}

export interface EnhanceResponse {
  enhanced_prompt: string;
  rationale: string;
}

/** Modelo del catálogo dinámico servido por el worker (vía gateway). */
export interface ModelInfo {
  id: string;
  name: string;
  mode: 'image' | 'video' | 'tts' | 'audio';
  engine: 'diffusers' | 'v1_bridge' | 'mock' | string;
  pipeline: string;
  repo: string;
  steps: number;
  vram_gb: number;
  family: string;
  available: boolean;
  notes: string;
}

export interface ModelsCatalog {
  models: ModelInfo[];
  device_name: string;
  cuda_available: boolean;
}

@Injectable({
  providedIn: 'root'
})
export class ApiService {
  // URL relativa: en dev la resuelve el proxy (proxy.conf.json) hacia :8080;
  // en producción Nginx enruta /api al gateway (ver nginx.conf).
  private baseUrl = '/api/v1';

  constructor(private http: HttpClient) {}

  checkHealth(): Observable<{ status: string; worker_connected: boolean }> {
    return this.http.get<{ status: string; worker_connected: boolean }>(`${this.baseUrl}/health`);
  }

  createJob(payload: CreateJobPayload): Observable<Job> {
    return this.http.post<Job>(`${this.baseUrl}/jobs/create`, payload);
  }

  listJobs(): Observable<Job[]> {
    return this.http.get<Job[]>(`${this.baseUrl}/jobs`);
  }

  getJobDetail(id: string): Observable<Job> {
    return this.http.get<Job>(`${this.baseUrl}/jobs/detail?id=${id}`);
  }

  cancelJob(id: string): Observable<Job> {
    return this.http.post<Job>(`${this.baseUrl}/jobs/cancel?id=${id}`, {});
  }

  getModels(): Observable<ModelsCatalog> {
    return this.http.get<ModelsCatalog>(`${this.baseUrl}/models`);
  }

  getGpuTelemetry(): Observable<GpuTelemetry> {
    return this.http.get<GpuTelemetry>(`${this.baseUrl}/gpu/telemetry`);
  }

  enhancePrompt(rawPrompt: string, targetModel: string, style = 'cinematic'): Observable<EnhanceResponse> {
    return this.http.post<EnhanceResponse>(`${this.baseUrl}/prompts/enhance`, {
      raw_prompt: rawPrompt,
      target_model: targetModel,
      style,
    });
  }
}
