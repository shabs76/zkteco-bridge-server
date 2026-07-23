import { HttpClient, HttpErrorResponse, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';
import { ConfigService } from './config.service';
import {
  AnswerResponse,
  CreateDeviceRequest,
  Dashboard,
  Device,
  PaginationResponse,
  Punch,
  PunchFilter,
  SetHRCredentialRequest,
  UpdateDeviceRequest,
} from './models';

/** Typed client for the gateway management API. GET helpers unwrap the envelope
 *  to the payload; mutations return the full AnswerResponse so callers can show
 *  the server message. */
@Injectable({ providedIn: 'root' })
export class ApiService {
  private http = inject(HttpClient);
  private cfg = inject(ConfigService);

  private url(path: string): string {
    return `${this.cfg.baseUrl()}${path}`;
  }

  // ---- Dashboard ----
  getDashboard(): Observable<Dashboard> {
    return this.http
      .get<AnswerResponse<Dashboard>>(this.url('/dashboard'))
      .pipe(map((r) => r.data));
  }

  // ---- Devices ----
  listDevices(page = 1, pageSize = 200): Observable<PaginationResponse<Device>> {
    const params = new HttpParams().set('page', page).set('pageSize', pageSize);
    return this.http.get<PaginationResponse<Device>>(this.url('/devices'), { params });
  }

  getDevice(id: string): Observable<Device> {
    return this.http
      .get<AnswerResponse<Device>>(this.url(`/devices/${id}`))
      .pipe(map((r) => r.data));
  }

  createDevice(body: CreateDeviceRequest): Observable<AnswerResponse<Device>> {
    return this.http.post<AnswerResponse<Device>>(this.url('/devices'), body);
  }

  updateDevice(id: string, body: UpdateDeviceRequest): Observable<AnswerResponse<Device>> {
    return this.http.patch<AnswerResponse<Device>>(this.url(`/devices/${id}`), body);
  }

  deleteDevice(id: string): Observable<AnswerResponse<null>> {
    return this.http.delete<AnswerResponse<null>>(this.url(`/devices/${id}`));
  }

  setCredential(id: string, body: SetHRCredentialRequest): Observable<AnswerResponse<Device>> {
    return this.http.post<AnswerResponse<Device>>(this.url(`/devices/${id}/hr-credential`), body);
  }

  requeueFailed(id: string): Observable<AnswerResponse<{ requeued: number }>> {
    return this.http.post<AnswerResponse<{ requeued: number }>>(
      this.url(`/devices/${id}/requeue`),
      {},
    );
  }

  // ---- Punches / logs ----
  listPunches(filter: PunchFilter): Observable<PaginationResponse<Punch>> {
    let params = new HttpParams()
      .set('page', filter.page ?? 1)
      .set('pageSize', filter.pageSize ?? 50);
    if (filter.deviceId) params = params.set('deviceId', filter.deviceId);
    if (filter.employeeNumber) params = params.set('employeeNumber', filter.employeeNumber);
    if (filter.status) params = params.set('status', filter.status);
    if (filter.from) params = params.set('from', filter.from);
    if (filter.to) params = params.set('to', filter.to);
    return this.http.get<PaginationResponse<Punch>>(this.url('/punches'), { params });
  }
}

/** Extracts a human-readable message from an HTTP failure. The gateway returns
 *  an AnswerResponse body with `message` on 4xx/5xx; network errors have none. */
export function apiError(err: unknown): string {
  if (err instanceof HttpErrorResponse) {
    const body = err.error as AnswerResponse<unknown> | string | null;
    if (body && typeof body === 'object' && 'message' in body && body.message) {
      return String(body.message);
    }
    if (err.status === 0) {
      return 'Cannot reach the gateway. Check the base URL and that the gateway is running.';
    }
    if (err.status === 401) {
      return 'Unauthorized — check the API key in Settings.';
    }
    return `Request failed (HTTP ${err.status}).`;
  }
  return 'Unexpected error.';
}
