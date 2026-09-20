import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { HttpTestingController } from '@angular/common/http/testing';

import { ApiService, CreateJobPayload, Job } from './api.service';

const JOB: Job = {
  id: 'job_1',
  mode: 'video',
  prompt: 'test prompt',
  model: 'Wan2.1-T2V-14B',
  width: 1280,
  height: 720,
  frames: 80,
  fps: 16,
  status: 'QUEUED',
  progress: 0,
  created_at: '2026-09-19T12:00:00Z',
};

describe('ApiService', () => {
  let service: ApiService;
  let httpMock: HttpTestingController;
  const BASE = '/api/v1';

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(ApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('checkHealth hace GET /health', () => {
    let res: { status: string; worker_connected: boolean } | undefined;
    service.checkHealth().subscribe((r) => (res = r));

    const req = httpMock.expectOne(`${BASE}/health`);
    expect(req.request.method).toBe('GET');
    req.flush({ status: 'healthy', worker_connected: true });
    expect(res?.worker_connected).toBe(true);
  });

  it('createJob hace POST /jobs/create con el payload', () => {
    const payload: CreateJobPayload = {
      mode: 'image',
      prompt: 'a cat',
      model: 'Flux.1-dev',
      width: 1024,
      height: 1024,
      frames: 0,
    };
    let res: Job | undefined;
    service.createJob(payload).subscribe((r) => (res = r));

    const req = httpMock.expectOne(`${BASE}/jobs/create`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(payload);
    req.flush({ ...JOB, mode: 'image', model: 'Flux.1-dev', frames: 0 });
    expect(res?.mode).toBe('image');
  });

  it('listJobs hace GET /jobs', () => {
    let res: Job[] | undefined;
    service.listJobs().subscribe((r) => (res = r));

    const req = httpMock.expectOne(`${BASE}/jobs`);
    req.flush([JOB]);
    expect(res?.length).toBe(1);
  });

  it('getJobDetail construye la URL con query param', () => {
    service.getJobDetail('job_9').subscribe();

    const req = httpMock.expectOne(`${BASE}/jobs/detail?id=job_9`);
    expect(req.request.method).toBe('GET');
    req.flush(JOB);
  });

  it('cancelJob hace POST /jobs/cancel?id=', () => {
    service.cancelJob('job_3').subscribe();

    const req = httpMock.expectOne(`${BASE}/jobs/cancel?id=job_3`);
    expect(req.request.method).toBe('POST');
    req.flush({ ...JOB, status: 'CANCELLED' });
  });

  it('getGpuTelemetry hace GET /gpu/telemetry', () => {
    let res: { device_name: string } | undefined;
    service.getGpuTelemetry().subscribe((r) => (res = r));

    const req = httpMock.expectOne(`${BASE}/gpu/telemetry`);
    req.flush({
      device_name: 'CPU / No CUDA',
      total_vram_mb: 0,
      used_vram_mb: 0,
      free_vram_mb: 0,
      gpu_utilization: 0,
      temperature_c: 0,
    });
    expect(res?.device_name).toBe('CPU / No CUDA');
  });

  it('enhancePrompt serializa snake_case y estilo por defecto', () => {
    let res: { enhanced_prompt: string } | undefined;
    service.enhancePrompt('a dragon', 'Wan 2.1').subscribe((r) => (res = r));

    const req = httpMock.expectOne(`${BASE}/prompts/enhance`);
    expect(req.request.body).toEqual({
      raw_prompt: 'a dragon',
      target_model: 'Wan 2.1',
      style: 'cinematic',
    });
    req.flush({ enhanced_prompt: 'better', rationale: 'why' });
    expect(res?.enhanced_prompt).toBe('better');
  });
});
