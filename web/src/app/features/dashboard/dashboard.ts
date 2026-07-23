import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ApiService, apiError } from '../../core/api.service';
import { Dashboard } from '../../core/models';
import { fmtRelative } from '../../core/format';

@Component({
  selector: 'app-dashboard',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink],
  template: `
    <div class="page">
      <div class="page-head">
        <div>
          <h1>Dashboard</h1>
          <p class="subtitle">Gateway status and per-device delivery health.</p>
        </div>
        <button class="btn" (click)="load()" [disabled]="loading()">
          @if (loading()) { <span class="spinner"></span> } @else { ⟳ } Refresh
        </button>
      </div>

      @if (error()) {
        <div class="banner">⚠ {{ error() }}</div>
      }

      @if (data(); as d) {
        <div class="kpi-grid">
          <div class="kpi accent-primary">
            <div class="label">Devices</div>
            <div class="value">{{ d.devices }}</div>
            <div class="sub">{{ d.activeDevices }} active</div>
          </div>
          <div class="kpi accent-warn">
            <div class="label">Pending</div>
            <div class="value">{{ d.pendingPunches }}</div>
            <div class="sub">oldest {{ rel(d.oldestPending) }}</div>
          </div>
          <div class="kpi accent-danger">
            <div class="label">Failed</div>
            <div class="value">{{ d.failedPunches }}</div>
            <div class="sub">need requeue</div>
          </div>
          <div class="kpi accent-ok">
            <div class="label">Delivered</div>
            <div class="value">{{ d.sentPunches }}</div>
            <div class="sub">last {{ rel(d.lastDelivery) }}</div>
          </div>
          <div class="kpi">
            <div class="label">Total punches</div>
            <div class="value">{{ d.totalPunches }}</div>
            <div class="sub">stored locally</div>
          </div>
        </div>

        <div class="card">
          <div class="card-head"><h3>Device health</h3></div>
          <div class="table-wrap">
            <table class="grid">
              <thead>
                <tr>
                  <th>Device</th><th>Mode</th><th>Status</th><th>HR cred</th>
                  <th>Last seen</th><th>Pending</th><th>Failed</th>
                </tr>
              </thead>
              <tbody>
                @for (h of d.deviceHealth; track h.id) {
                  <tr class="clickable" [routerLink]="['/devices', h.id]">
                    <td>
                      <div>{{ h.name }}</div>
                      <div class="faint mono">{{ h.serialNumber || '—' }}</div>
                    </td>
                    <td><span class="badge">{{ h.mode }}</span></td>
                    <td>
                      @if (!h.isActive) {
                        <span class="badge">inactive</span>
                      } @else if (h.online) {
                        <span class="badge ok"><span class="dot ok"></span> online</span>
                      } @else {
                        <span class="badge warn"><span class="dot warn"></span> offline</span>
                      }
                    </td>
                    <td>
                      @if (h.hrCredentialSet) { <span class="badge ok">set</span> }
                      @else { <span class="badge danger">missing</span> }
                    </td>
                    <td class="nowrap muted">{{ rel(h.lastSeenAt) }}</td>
                    <td>@if (h.pending > 0) { <span class="badge warn">{{ h.pending }}</span> } @else { 0 }</td>
                    <td>@if (h.failed > 0) { <span class="badge danger">{{ h.failed }}</span> } @else { 0 }</td>
                  </tr>
                } @empty {
                  <tr><td colspan="7" class="empty">No devices registered yet.</td></tr>
                }
              </tbody>
            </table>
          </div>
        </div>
      } @else if (loading()) {
        <div class="card"><div class="loading-row"><span class="spinner"></span> Loading dashboard…</div></div>
      }
    </div>
  `,
})
export class DashboardPage implements OnInit {
  private api = inject(ApiService);
  protected data = signal<Dashboard | null>(null);
  protected loading = signal(false);
  protected error = signal('');

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.error.set('');
    this.api.getDashboard().subscribe({
      next: (d) => {
        this.data.set(d);
        this.loading.set(false);
      },
      error: (e) => {
        this.error.set(apiError(e));
        this.loading.set(false);
      },
    });
  }

  protected rel = fmtRelative;
}
