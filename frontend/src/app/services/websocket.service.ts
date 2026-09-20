import { Injectable, signal } from '@angular/core';
import { Observable, Subject } from 'rxjs';

export interface JobUpdate {
  type: string;
  data: {
    id: string;
    mode: string;
    prompt: string;
    model: string;
    status: string;
    progress: number;
    output_path?: string;
  };
}

@Injectable({
  providedIn: 'root'
})
export class WebSocketService {
  private socket!: WebSocket;
  private jobUpdates$ = new Subject<JobUpdate>();
  
  public isConnected = signal<boolean>(false);
  public lastJob = signal<any>(null);

  // URL relativa al host actual: misma-origin en dev (proxy) y en producción (Nginx).
  connect(url?: string): Observable<JobUpdate> {
    const wsUrl = url ?? `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/telemetry`;
    this.socket = new WebSocket(wsUrl);

    this.socket.onopen = () => {
      console.log('📡 [WebSocket] Conectado a Brain-Master Go Gateway');
      this.isConnected.set(true);
    };

    this.socket.onmessage = (event) => {
      try {
        const message: JobUpdate = JSON.parse(event.data);
        if (message.type === 'JOB_UPDATE') {
          this.lastJob.set(message.data);
        }
        this.jobUpdates$.next(message);
      } catch (err) {
        console.error('Error parseando mensaje WebSocket:', err);
      }
    };

    this.socket.onclose = () => {
      console.warn('⚠️ [WebSocket] Desconectado del Gateway');
      this.isConnected.set(false);
    };

    this.socket.onerror = (error) => {
      console.error('❌ [WebSocket] Error:', error);
    };

    return this.jobUpdates$.asObservable();
  }

  disconnect() {
    if (this.socket) {
      this.socket.close();
    }
  }
}
