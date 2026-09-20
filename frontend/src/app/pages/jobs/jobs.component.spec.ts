import { TestBed, ComponentFixture } from '@angular/core/testing';
import { TranslateService } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';

import { JobsComponent } from './jobs.component';
import { ApiService, Job } from '../../services/api.service';
import { WebSocketService } from '../../services/websocket.service';

function makeJob(over: Partial<Job>): Job {
  return {
    id: 'job_x',
    mode: 'video',
    prompt: 'p',
    model: 'm',
    width: 1280,
    height: 720,
    frames: 80,
    status: 'QUEUED',
    progress: 0,
    created_at: '2026-09-19T12:00:00Z',
    ...over,
  };
}

describe('JobsComponent', () => {
  let fixture: ComponentFixture<JobsComponent>;
  let comp: JobsComponent;
  let listJobsSpy: ReturnType<
    typeof vi.fn<() => Observable<Job[]>>
  >;
  let cancelJobSpy: ReturnType<
    typeof vi.fn<(id: string) => Observable<Job>>
  >;

  beforeEach(() => {
    listJobsSpy = vi.fn<() => Observable<Job[]>>(() => of([]));
    cancelJobSpy = vi.fn<(id: string) => Observable<Job>>(() => of({} as Job));

    const apiStub: Partial<ApiService> = {
      listJobs: () => listJobsSpy(),
      cancelJob: (id: string) => cancelJobSpy(id),
    };

    TestBed.configureTestingModule({
      imports: [JobsComponent],
      providers: [
        { provide: ApiService, useValue: apiStub },
        {
          provide: WebSocketService,
          useValue: { connect: () => of() },
        },
        {
          provide: TranslateService,
          useValue: { get: (k: string) => of(k), instant: (k: string) => k },
        },
      ],
    });

    fixture = TestBed.createComponent(JobsComponent);
    comp = fixture.componentInstance;
  });

  it('carga los jobs al inicializar', () => {
    listJobsSpy.mockReturnValue(
      of([makeJob({ id: 'a', status: 'COMPLETED', progress: 100 })]),
    );
    comp.ngOnInit();
    expect(comp.jobs().length).toBe(1);
    expect(comp.jobs()[0].id).toBe('a');
  });

  it('filtra por estado con el computed', () => {
    listJobsSpy.mockReturnValue(
      of([
        makeJob({ id: 'a', status: 'COMPLETED' }),
        makeJob({ id: 'b', status: 'PROCESSING' }),
        makeJob({ id: 'c', status: 'PROCESSING' }),
      ]),
    );
    comp.ngOnInit();

    expect(comp.filteredJobs().length).toBe(3);

    comp.statusFilter.set('PROCESSING');
    expect(comp.filteredJobs().length).toBe(2);
    expect(comp.filteredJobs().every((j) => j.status === 'PROCESSING')).toBe(true);

    comp.statusFilter.set('COMPLETED');
    expect(comp.filteredJobs().length).toBe(1);
  });

  it('cuenta los jobs activos (QUEUED + PROCESSING)', () => {
    listJobsSpy.mockReturnValue(
      of([
        makeJob({ id: 'a', status: 'PROCESSING' }),
        makeJob({ id: 'b', status: 'QUEUED' }),
        makeJob({ id: 'c', status: 'COMPLETED' }),
        makeJob({ id: 'd', status: 'FAILED' }),
      ]),
    );
    comp.ngOnInit();
    expect(comp.activeCount()).toBe(2);
  });

  it('cancel llama a la API y refresca la lista', () => {
    comp.ngOnInit(); // 1ª llamada a listJobs
    const job = makeJob({ id: 'job_9', status: 'PROCESSING' });
    comp.cancel(job);

    expect(cancelJobSpy).toHaveBeenCalledWith('job_9');
    expect(listJobsSpy).toHaveBeenCalledTimes(2); // init + refresh
  });

  it('mapea severidades de tags correctamente', () => {
    expect(comp.severity('COMPLETED')).toBe('success');
    expect(comp.severity('PROCESSING')).toBe('info');
    expect(comp.severity('QUEUED')).toBe('warn');
    expect(comp.severity('FAILED')).toBe('danger');
    expect(comp.severity('CANCELLED')).toBe('secondary');
  });

  it('construye la clave i18n del estado', () => {
    expect(comp.statusKey('COMPLETED')).toBe('JOBS.STATUS_COMPLETED');
  });

  it('openDetail selecciona el job', () => {
    const job = makeJob({ id: 'job_d' });
    comp.openDetail(job);
    expect(comp.selectedJob()?.id).toBe('job_d');
  });
});
