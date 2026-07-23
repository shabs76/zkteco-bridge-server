import { ChangeDetectionStrategy, Component, OnInit, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService, apiError } from '../../core/api.service';
import { Device, DeliveryStatus, Pagination, Punch } from '../../core/models';
import { fmtDateTime } from '../../core/format';

const PAGE_SIZE = 50;

@Component({
  selector: 'app-logs',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  template: `
    <div class="page">
      <div class="page-head">
        <div>
          <h1>Logs</h1>
          <p class="subtitle">Stored attendance punches and their delivery status to HR.</p>
        </div>
        <button class="btn" (click)="reload()" [disabled]="loading()">
          @if (loading()) { <span class="spinner"></span> } @else { ⟳ } Refresh
        </button>
      </div>

      @if (error()) { <div class="banner">⚠ {{ error() }}</div> }

      <div class="card card-pad">
        <div class="row">
          <div class="field" style="margin:0; min-width:180px">
            <label>Device</label>
            <select [(ngModel)]="deviceIdFilter">
              <option value="">All devices</option>
              @for (d of devices(); track d.id) {
                <option [value]="d.id">{{ d.name }}</option>
              }
            </select>
          </div>
          <div class="field" style="margin:0; min-width:150px">
            <label>Status</label>
            <select [(ngModel)]="status">
              <option value="">Any status</option>
              <option value="Pending">Pending</option>
              <option value="Sent">Sent</option>
              <option value="Failed">Failed</option>
            </select>
          </div>
          <div class="field" style="margin:0; min-width:150px">
            <label>Employee #</label>
            <input [(ngModel)]="employeeNumber" class="mono" placeholder="e.g. 1024" />
          </div>
          <div class="field" style="margin:0">
            <label>From</label>
            <input type="date" [(ngModel)]="from" />
          </div>
          <div class="field" style="margin:0">
            <label>To</label>
            <input type="date" [(ngModel)]="to" />
          </div>
          <div class="field" style="margin:0; align-self:flex-end">
            <button class="btn primary" (click)="apply()">Apply</button>
          </div>
          <div class="field" style="margin:0; align-self:flex-end">
            <button class="btn ghost" (click)="clear()">Clear</button>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="table-wrap">
          <table class="grid">
            <thead>
              <tr>
                <th>Employee</th><th>Type</th><th>Event time</th><th>Device</th>
                <th>Source</th><th>Delivery</th><th>Attempts</th><th>Sent at</th>
              </tr>
            </thead>
            <tbody>
              @for (p of punches(); track p.id) {
                <tr>
                  <td class="mono">{{ p.employeeNumber }}</td>
                  <td>
                    @if (p.logType === 'IN') { <span class="badge ok">IN</span> }
                    @else { <span class="badge primary">OUT</span> }
                  </td>
                  <td class="nowrap">{{ dt(p.eventTime) }}</td>
                  <td class="mono muted">{{ p.deviceSerial || '—' }}</td>
                  <td class="muted">{{ p.source }}</td>
                  <td>
                    @switch (p.deliveryStatus) {
                      @case ('Sent') { <span class="badge ok">Sent</span> }
                      @case ('Failed') { <span class="badge danger" [title]="p.lastError">Failed</span> }
                      @default { <span class="badge warn">Pending</span> }
                    }
                  </td>
                  <td>{{ p.deliveryAttempts }}</td>
                  <td class="nowrap muted">{{ dt(p.sentAt) }}</td>
                </tr>
              } @empty {
                @if (!loading()) {
                  <tr><td colspan="8"><div class="empty">No punches match these filters.</div></td></tr>
                }
              }
            </tbody>
          </table>
        </div>
        @if (loading()) {
          <div class="loading-row"><span class="spinner"></span> Loading logs…</div>
        } @else if (page(); as pg) {
          <div class="pager">
            <span>{{ pg.totalItems }} punch(es) · page {{ pg.page }} of {{ pg.totalPages || 1 }}</span>
            <button class="btn sm" (click)="go(pg.page - 1)" [disabled]="pg.page <= 1">‹ Prev</button>
            <button class="btn sm" (click)="go(pg.page + 1)" [disabled]="pg.page >= pg.totalPages">Next ›</button>
          </div>
        }
      </div>
    </div>
  `,
})
export class LogsPage implements OnInit {
  private api = inject(ApiService);

  /** Optional deep-link from the device detail page. */
  readonly deviceId = input<string>('');

  protected punches = signal<Punch[]>([]);
  protected devices = signal<Device[]>([]);
  protected page = signal<Pagination | null>(null);
  protected loading = signal(false);
  protected error = signal('');

  // Filter model (mutated by the form, committed on Apply).
  protected deviceIdFilter = '';
  protected status: DeliveryStatus | '' = '';
  protected employeeNumber = '';
  protected from = '';
  protected to = '';
  private current = 1;

  ngOnInit(): void {
    this.deviceIdFilter = this.deviceId();
    this.api.listDevices().subscribe({ next: (r) => this.devices.set(r.data), error: () => {} });
    this.load(1);
  }

  apply(): void {
    this.load(1);
  }

  clear(): void {
    this.deviceIdFilter = '';
    this.status = '';
    this.employeeNumber = '';
    this.from = '';
    this.to = '';
    this.load(1);
  }

  go(p: number): void {
    this.load(p);
  }

  reload(): void {
    this.load(this.current);
  }

  private load(p: number): void {
    this.current = p;
    this.loading.set(true);
    this.error.set('');
    this.api
      .listPunches({
        page: p,
        pageSize: PAGE_SIZE,
        deviceId: this.deviceIdFilter,
        employeeNumber: this.employeeNumber.trim(),
        status: this.status,
        from: this.from,
        to: this.to,
      })
      .subscribe({
        next: (r) => {
          this.punches.set(r.data);
          this.page.set(r.pagination);
          this.loading.set(false);
        },
        error: (e) => {
          this.error.set(apiError(e));
          this.loading.set(false);
        },
      });
  }

  protected dt = fmtDateTime;
}
