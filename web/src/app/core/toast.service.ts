import { Injectable, signal } from '@angular/core';

export type ToastKind = 'success' | 'error' | 'info';

export interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
}

@Injectable({ providedIn: 'root' })
export class ToastService {
  readonly toasts = signal<Toast[]>([]);
  private seq = 0;

  success(message: string): void {
    this.push('success', message);
  }

  error(message: string): void {
    this.push('error', message, 6500);
  }

  info(message: string): void {
    this.push('info', message);
  }

  dismiss(id: number): void {
    this.toasts.update((list) => list.filter((t) => t.id !== id));
  }

  private push(kind: ToastKind, message: string, ttl = 4000): void {
    const id = ++this.seq;
    this.toasts.update((list) => [...list, { id, kind, message }]);
    setTimeout(() => this.dismiss(id), ttl);
  }
}
