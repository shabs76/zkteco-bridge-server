import { ChangeDetectionStrategy, Component, OnInit, inject, input, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { ApiService, apiError } from '../../core/api.service';
import { ToastService } from '../../core/toast.service';
import { Device, Punch } from '../../core/models';
import { fmtDateTime, fmtRelative } from '../../core/format';
import { DeviceForm } from './device-form';
import { CredentialForm } from './credential-form';

@Component({
  selector: 'app-device-detail',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, DeviceForm, CredentialForm],
  template: `
    <div class="page">
      <div class="page-head">
        <div>
          <a routerLink="/devices" class="muted">← Devices</a>
          <h1>{{ device()?.name ?? 'Device' }}</h1>
          <p class="subtitle mono">{{ id() }}</p>
        </div>
        @if (device(); as d) {
          <div class="row">
            <button class="btn" (click)="reload()">⟳ Refresh</button>
            <button class="btn" (click)="showEdit.set(true)">Edit</button>
            <button class="btn" (click)="showCred.set(true)">
              {{ d.hrCredentialSet ? 'Rotate' : 'Set' }} credential
            </button>
            <button class="btn" (click)="toggleActive(d)" [disabled]="busy()">
              {{ d.isActive ? 'Deactivate' : 'Activate' }}
            </button>
            <button class="btn" (click)="requeue(d)" [disabled]="busy()">Requeue failed</button>
            <button class="btn danger" (click)="remove(d)" [disabled]="busy()">Delete</button>
          </div>
        }
      </div>

      @if (error()) { <div class="banner">⚠ {{ error() }}</div> }

      @if (device(); as d) {
        <div class="card card-pad">
          <dl class="dl">
            <dt>Status</dt>
            <dd>
              @if (d.isActive) { <span class="badge ok">active</span> } @else { <span class="badge">inactive</span> }
            </dd>
            <dt>Mode</dt><dd><span class="badge">{{ d.mode }}</span></dd>
            <dt>Serial number</dt><dd class="mono">{{ d.serialNumber || '—' }}</dd>
            <dt>Address</dt><dd class="mono">{{ d.ipAddress ? d.ipAddress + ':' + d.port : '—' }}</dd>
            <dt>Timezone</dt><dd>{{ d.timezone || '—' }}</dd>
            <dt>Poll interval</dt><dd>{{ d.pollIntervalSeconds ? d.pollIntervalSeconds + 's' : 'default' }}</dd>
            <dt>Punch state map</dt><dd class="mono">{{ d.punchStateMap || 'default' }}</dd>
            <dt>HR credential</dt>
            <dd>
              @if (d.hrCredentialSet) {
                <span class="badge ok">set</span>
                <span class="mono muted">&nbsp;id {{ d.hrDeviceId || '—' }} · key {{ d.hrKeyPrefix }}…</span>
              } @else { <span class="badge danger">missing</span> }
            </dd>
            <dt>Last seen</dt><dd>{{ dt(d.lastSeenAt) }} <span class="faint">({{ rel(d.lastSeenAt) }})</span></dd>
            <dt>Last pull</dt><dd>{{ dt(d.lastPullAt) }}</dd>
            @if (d.lastPullError) { <dt>Last pull error</dt><dd class="mono" style="color:var(--danger)">{{ d.lastPullError }}</dd> }
            <dt>Registered</dt><dd>{{ dt(d.createdAt) }}</dd>
          </dl>
        </div>

        <div class="card">
          <div class="card-head">
            <h3>Recent logs</h3>
            <a [routerLink]="['/logs']" [queryParams]="{ deviceId: d.id }">View all →</a>
          </div>
          <div class="table-wrap">
            <table class="grid">
              <thead>
                <tr><th>Employee</th><th>Type</th><th>Event time</th><th>Source</th><th>Delivery</th><th>Attempts</th></tr>
              </thead>
              <tbody>
                @for (p of logs(); track p.id) {
                  <tr>
                    <td class="mono">{{ p.employeeNumber }}</td>
                    <td>
                      @if (p.logType === 'IN') { <span class="badge ok">IN</span> }
                      @else { <span class="badge primary">OUT</span> }
                    </td>
                    <td class="nowrap">{{ dt(p.eventTime) }}</td>
                    <td class="muted">{{ p.source }}</td>
                    <td>
                      @switch (p.deliveryStatus) {
                        @case ('Sent') { <span class="badge ok">Sent</span> }
                        @case ('Failed') { <span class="badge danger" [title]="p.lastError">Failed</span> }
                        @default { <span class="badge warn">Pending</span> }
                      }
                    </td>
                    <td>{{ p.deliveryAttempts }}</td>
                  </tr>
                } @empty {
                  @if (!logsLoading()) { <tr><td colspan="6" class="empty">No logs from this device yet.</td></tr> }
                }
              </tbody>
            </table>
          </div>
          @if (logsLoading()) { <div class="loading-row"><span class="spinner"></span> Loading logs…</div> }
        </div>
      } @else if (loading()) {
        <div class="card"><div class="loading-row"><span class="spinner"></span> Loading device…</div></div>
      }
    </div>

    @if (showEdit() && device()) {
      <app-device-form [device]="device()" (close)="showEdit.set(false)" (saved)="onSaved($event)" />
    }
    @if (showCred() && device()) {
      <app-credential-form [device]="device()!" (close)="showCred.set(false)" (saved)="onSaved($event)" />
    }
  `,
})
export class DeviceDetailPage implements OnInit {
  private api = inject(ApiService);
  private toast = inject(ToastService);
  private router = inject(Router);

  readonly id = input.required<string>();

  protected device = signal<Device | null>(null);
  protected logs = signal<Punch[]>([]);
  protected loading = signal(false);
  protected logsLoading = signal(false);
  protected busy = signal(false);
  protected error = signal('');
  protected showEdit = signal(false);
  protected showCred = signal(false);

  ngOnInit(): void {
    this.reload();
  }

  reload(): void {
    this.loading.set(true);
    this.error.set('');
    this.api.getDevice(this.id()).subscribe({
      next: (d) => {
        this.device.set(d);
        this.loading.set(false);
        this.loadLogs();
      },
      error: (e) => {
        this.error.set(apiError(e));
        this.loading.set(false);
      },
    });
  }

  private loadLogs(): void {
    this.logsLoading.set(true);
    this.api.listPunches({ deviceId: this.id(), pageSize: 25 }).subscribe({
      next: (r) => {
        this.logs.set(r.data);
        this.logsLoading.set(false);
      },
      error: () => this.logsLoading.set(false),
    });
  }

  onSaved(d: Device): void {
    this.showEdit.set(false);
    this.showCred.set(false);
    this.device.set(d);
  }

  toggleActive(d: Device): void {
    this.busy.set(true);
    this.api.updateDevice(d.id, { isActive: !d.isActive }).subscribe({
      next: (r) => {
        this.busy.set(false);
        this.device.set(r.data);
        this.toast.success(r.data.isActive ? 'Device activated' : 'Device deactivated');
      },
      error: (e) => {
        this.busy.set(false);
        this.toast.error(apiError(e));
      },
    });
  }

  requeue(d: Device): void {
    this.busy.set(true);
    this.api.requeueFailed(d.id).subscribe({
      next: (r) => {
        this.busy.set(false);
        this.toast.success(`Requeued ${r.data?.requeued ?? 0} failed punch(es)`);
        this.loadLogs();
      },
      error: (e) => {
        this.busy.set(false);
        this.toast.error(apiError(e));
      },
    });
  }

  remove(d: Device): void {
    if (!confirm(`Delete "${d.name}"? Stored punches for it remain in the database.`)) return;
    this.busy.set(true);
    this.api.deleteDevice(d.id).subscribe({
      next: () => {
        this.busy.set(false);
        this.toast.success('Device deleted');
        this.router.navigate(['/devices']);
      },
      error: (e) => {
        this.busy.set(false);
        this.toast.error(apiError(e));
      },
    });
  }

  protected dt = fmtDateTime;
  protected rel = fmtRelative;
}
