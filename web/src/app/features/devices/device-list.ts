import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ApiService, apiError } from '../../core/api.service';
import { Device } from '../../core/models';
import { fmtRelative } from '../../core/format';
import { DeviceForm } from './device-form';

@Component({
  selector: 'app-device-list',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, DeviceForm],
  template: `
    <div class="page">
      <div class="page-head">
        <div>
          <h1>Devices</h1>
          <p class="subtitle">Registered ZKTeco units and their delivery configuration.</p>
        </div>
        <div class="row">
          <button class="btn" (click)="load()" [disabled]="loading()">
            @if (loading()) { <span class="spinner"></span> } @else { ⟳ } Refresh
          </button>
          <button class="btn primary" (click)="showCreate.set(true)">+ Register device</button>
        </div>
      </div>

      @if (error()) { <div class="banner">⚠ {{ error() }}</div> }

      <div class="card">
        <div class="table-wrap">
          <table class="grid">
            <thead>
              <tr>
                <th>Name</th><th>Serial</th><th>Mode</th><th>Address</th>
                <th>HR cred</th><th>Active</th><th>Last seen</th>
              </tr>
            </thead>
            <tbody>
              @for (d of devices(); track d.id) {
                <tr class="clickable" [routerLink]="['/devices', d.id]">
                  <td>{{ d.name }}</td>
                  <td class="mono">{{ d.serialNumber || '—' }}</td>
                  <td><span class="badge">{{ d.mode }}</span></td>
                  <td class="mono muted">{{ d.ipAddress ? d.ipAddress + ':' + d.port : '—' }}</td>
                  <td>
                    @if (d.hrCredentialSet) { <span class="badge ok">set</span> }
                    @else { <span class="badge danger">missing</span> }
                  </td>
                  <td>
                    @if (d.isActive) { <span class="badge ok">yes</span> }
                    @else { <span class="badge">no</span> }
                  </td>
                  <td class="nowrap muted">{{ rel(d.lastSeenAt) }}</td>
                </tr>
              } @empty {
                @if (!loading()) {
                  <tr><td colspan="7">
                    <div class="empty">
                      <div class="big">▣</div>
                      No devices yet. Register your first ZKTeco unit to start collecting punches.
                    </div>
                  </td></tr>
                }
              }
            </tbody>
          </table>
        </div>
        @if (loading()) {
          <div class="loading-row"><span class="spinner"></span> Loading devices…</div>
        }
      </div>
    </div>

    @if (showCreate()) {
      <app-device-form (close)="showCreate.set(false)" (saved)="onCreated()" />
    }
  `,
})
export class DeviceListPage implements OnInit {
  private api = inject(ApiService);
  protected devices = signal<Device[]>([]);
  protected loading = signal(false);
  protected error = signal('');
  protected showCreate = signal(false);

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.error.set('');
    this.api.listDevices().subscribe({
      next: (r) => {
        this.devices.set(r.data);
        this.loading.set(false);
      },
      error: (e) => {
        this.error.set(apiError(e));
        this.loading.set(false);
      },
    });
  }

  onCreated(): void {
    this.showCreate.set(false);
    this.load();
  }

  protected rel = fmtRelative;
}
