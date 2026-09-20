import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { ProgressBarModule } from 'primeng/progressbar';
import { SelectModule } from 'primeng/select';
import { DialogModule } from 'primeng/dialog';
import { CardModule } from 'primeng/card';
import { ApiService, Job } from '../../services/api.service';
import { WebSocketService } from '../../services/websocket.service';

type StatusFilter = 'ALL' | 'QUEUED' | 'PROCESSING' | 'COMPLETED' | 'FAILED' | 'CANCELLED';

@Component({
  selector: 'app-jobs',
  standalone: true,
  imports: [
    DatePipe,
    DecimalPipe,
    FormsModule,
    TranslatePipe,
    ButtonModule,
    TooltipModule,
    TableModule,
    TagModule,
    ProgressBarModule,
    SelectModule,
    DialogModule,
    CardModule,
  ],
  templateUrl: './jobs.component.html',
})
export class JobsComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly wsService = inject(WebSocketService);

  readonly jobs = signal<Job[]>([]);
  readonly selectedJob = signal<Job | null>(null);
  readonly statusFilter = signal<StatusFilter>('ALL');

  readonly statusOptions = [
    { label: 'JOBS.FILTER_ALL', value: 'ALL' },
    { label: 'JOBS.STATUS_QUEUED', value: 'QUEUED' },
    { label: 'JOBS.STATUS_PROCESSING', value: 'PROCESSING' },
    { label: 'JOBS.STATUS_COMPLETED', value: 'COMPLETED' },
    { label: 'JOBS.STATUS_FAILED', value: 'FAILED' },
    { label: 'JOBS.STATUS_CANCELLED', value: 'CANCELLED' },
  ];

  readonly filteredJobs = computed(() => {
    const f = this.statusFilter();
    const jobs = this.jobs();
    return f === 'ALL' ? jobs : jobs.filter((j) => j.status === f);
  });

  readonly activeCount = computed(
    () => this.jobs().filter((j) => j.status === 'PROCESSING' || j.status === 'QUEUED').length,
  );

  ngOnInit(): void {
    this.refresh();
  }

  refresh(): void {
    this.api.listJobs().subscribe({
      next: (jobs) => this.jobs.set(jobs),
      error: (err) => console.error('Error cargando jobs:', err),
    });
  }

  cancel(job: Job): void {
    this.api.cancelJob(job.id).subscribe({
      next: () => this.refresh(),
      error: (err) => console.error('Cancel falló:', err),
    });
  }

  openDetail(job: Job): void {
    this.selectedJob.set(job);
  }

  severity(status: string): 'success' | 'warn' | 'danger' | 'secondary' | 'info' {
    switch (status) {
      case 'COMPLETED': return 'success';
      case 'PROCESSING': return 'info';
      case 'QUEUED': return 'warn';
      case 'FAILED': return 'danger';
      default: return 'secondary';
    }
  }

  statusKey(status: string): string {
    return `JOBS.STATUS_${status}`;
  }
}
