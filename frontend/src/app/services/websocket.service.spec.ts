import { TestBed } from '@angular/core/testing';

import { WebSocketService, JobUpdate } from './websocket.service';

/** WebSocket falso: captura los handlers sin abrir conexiones reales. */
class FakeWebSocket {
  static last: FakeWebSocket | null = null;
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  closed = false;

  constructor(public url: string) {
    FakeWebSocket.last = this;
  }

  close(): void {
    this.closed = true;
    this.onclose?.();
  }
}

describe('WebSocketService', () => {
  let service: WebSocketService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(WebSocketService);
    (window as any).WebSocket = FakeWebSocket;
  });

  afterEach(() => {
    try {
      service.disconnect();
    } catch {
      /* noop */
    }
  });

  it('construye la URL relativa al host actual', () => {
    service.connect();
    expect(FakeWebSocket.last?.url).toBe(`ws://${location.host}/ws/telemetry`);
  });

  it('respeta una URL explícita', () => {
    service.connect('ws://custom:8080/ws');
    expect(FakeWebSocket.last?.url).toBe('ws://custom:8080/ws');
  });

  it('marca isConnected al abrir y desconecta al cerrar', () => {
    service.connect();
    const sock = FakeWebSocket.last!;

    expect(service.isConnected()).toBe(false);
    sock.onopen?.();
    expect(service.isConnected()).toBe(true);

    sock.onclose?.();
    expect(service.isConnected()).toBe(false);
  });

  it('emite JOB_UPDATE por el observable y actualiza lastJob', () => {
    const received: JobUpdate[] = [];
    service.connect().subscribe((u) => received.push(u));
    const sock = FakeWebSocket.last!;

    sock.onopen?.();
    sock.onmessage?.({
      data: JSON.stringify({
        type: 'JOB_UPDATE',
        data: { id: 'j1', status: 'PROCESSING', progress: 40 },
      }),
    });

    expect(received.length).toBe(1);
    expect(received[0].data.id).toBe('j1');
    expect(service.lastJob()?.progress).toBe(40);
  });

  it('ignora mensajes que no son JOB_UPDATE sin dejar de emitirlos', () => {
    const received: JobUpdate[] = [];
    service.connect().subscribe((u) => received.push(u));
    const sock = FakeWebSocket.last!;

    sock.onmessage?.({ data: JSON.stringify({ type: 'OTHER', data: {} }) });

    expect(received.length).toBe(1);
    expect(service.lastJob()).toBeNull();
  });

  it('no revienta con payloads JSON inválidos', () => {
    const received: JobUpdate[] = [];
    service.connect().subscribe((u) => received.push(u));
    const sock = FakeWebSocket.last!;

    expect(() => sock.onmessage?.({ data: '{invalid' })).not.toThrow();
    expect(received.length).toBe(0);
  });

  it('disconnect cierra el socket sin error aunque no haya conexión', () => {
    expect(() => service.disconnect()).not.toThrow();
    service.connect();
    expect(() => service.disconnect()).not.toThrow();
    expect(FakeWebSocket.last?.closed).toBe(true);
  });
});
