import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService, apiError } from '../../core/api.service';
import { ToastService } from '../../core/toast.service';
import { CreateDeviceRequest, Device, DeviceMode, UpdateDeviceRequest } from '../../core/models';

interface FormModel {
  name: string;
  serialNumber: string;
  ipAddress: string;
  port: number;
  commPassword: number;
  mode: DeviceMode;
  timezone: string;
  pollIntervalSeconds: number;
  punchStateMap: string;
  isActive: boolean;
  hrDeviceId: string;
  hrDeviceKey: string;
}

/** Modal used to register a new device or edit an existing one. In edit mode the
 *  HR credential is managed separately (rotate action), so it is omitted here. */
@Component({
  selector: 'app-device-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  template: `
    <div class="modal-backdrop" (click)="onBackdrop($event)">
      <div class="modal wide">
        <div class="modal-head">
          <h3>{{ device() ? 'Edit device' : 'Register device' }}</h3>
          <button class="btn ghost icon" (click)="close.emit()" aria-label="Close">×</button>
        </div>
        <div class="modal-body">
          <div class="form-grid">
            <div class="field full">
              <label>Name *</label>
              <input [(ngModel)]="form.name" placeholder="Main entrance reader" />
            </div>

            <div class="field">
              <label>Mode</label>
              <select [(ngModel)]="form.mode">
                <option value="both">both (pull + push)</option>
                <option value="pull">pull</option>
                <option value="push">push</option>
              </select>
              <div class="hint">pull needs an IP; push needs a serial number.</div>
            </div>
            <div class="field">
              <label>Timezone</label>
              <input [(ngModel)]="form.timezone" placeholder="e.g. Africa/Dar_es_Salaam" />
            </div>

            <div class="field">
              <label>Serial number</label>
              <input [(ngModel)]="form.serialNumber" class="mono" placeholder="how the device identifies itself" />
            </div>
            <div class="field">
              <label>IP address</label>
              <input [(ngModel)]="form.ipAddress" class="mono" placeholder="192.168.1.50" />
            </div>

            <div class="field">
              <label>Port</label>
              <input type="number" [(ngModel)]="form.port" placeholder="4370" />
            </div>
            <div class="field">
              <label>Comm password</label>
              <input type="number" [(ngModel)]="form.commPassword" placeholder="0 = none" />
            </div>

            <div class="field">
              <label>Poll interval (seconds)</label>
              <input type="number" [(ngModel)]="form.pollIntervalSeconds" placeholder="0 = default" />
            </div>
            <div class="field">
              <label>Punch state map</label>
              <input [(ngModel)]="form.punchStateMap" class="mono" placeholder='{"0":"IN","1":"OUT"}' />
            </div>

            @if (!device()) {
              <div class="field">
                <label>HR device ID</label>
                <input [(ngModel)]="form.hrDeviceId" class="mono" placeholder="from the HR service" />
              </div>
              <div class="field">
                <label>HR device key</label>
                <input [(ngModel)]="form.hrDeviceKey" type="password" class="mono" placeholder="encrypted at rest" />
              </div>
            } @else {
              <div class="field full check-row">
                <input id="active" type="checkbox" [(ngModel)]="form.isActive" />
                <label for="active">Device is active</label>
              </div>
            }
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" (click)="close.emit()" [disabled]="saving()">Cancel</button>
          <button class="btn primary" (click)="save()" [disabled]="saving() || !form.name.trim()">
            @if (saving()) { <span class="spinner"></span> }
            {{ device() ? 'Save changes' : 'Register' }}
          </button>
        </div>
      </div>
    </div>
  `,
})
export class DeviceForm implements OnInit {
  private api = inject(ApiService);
  private toast = inject(ToastService);

  readonly device = input<Device | null>(null);
  readonly close = output<void>();
  readonly saved = output<Device>();

  protected saving = signal(false);
  protected form: FormModel = {
    name: '',
    serialNumber: '',
    ipAddress: '',
    port: 4370,
    commPassword: 0,
    mode: 'both',
    timezone: '',
    pollIntervalSeconds: 0,
    punchStateMap: '',
    isActive: true,
    hrDeviceId: '',
    hrDeviceKey: '',
  };

  ngOnInit(): void {
    const d = this.device();
    if (d) {
      this.form = {
        name: d.name,
        serialNumber: d.serialNumber,
        ipAddress: d.ipAddress,
        port: d.port || 4370,
        commPassword: 0,
        mode: d.mode,
        timezone: d.timezone,
        pollIntervalSeconds: d.pollIntervalSeconds,
        punchStateMap: d.punchStateMap,
        isActive: d.isActive,
        hrDeviceId: '',
        hrDeviceKey: '',
      };
    }
  }

  onBackdrop(e: MouseEvent): void {
    if ((e.target as HTMLElement).classList.contains('modal-backdrop')) this.close.emit();
  }

  save(): void {
    const d = this.device();
    this.saving.set(true);
    if (d) {
      const body: UpdateDeviceRequest = {
        name: this.form.name.trim(),
        serialNumber: this.form.serialNumber.trim(),
        ipAddress: this.form.ipAddress.trim(),
        port: Number(this.form.port),
        commPassword: Number(this.form.commPassword),
        mode: this.form.mode,
        timezone: this.form.timezone.trim(),
        punchStateMap: this.form.punchStateMap.trim(),
        pollIntervalSeconds: Number(this.form.pollIntervalSeconds),
        isActive: this.form.isActive,
      };
      this.api.updateDevice(d.id, body).subscribe({
        next: (r) => this.done(r.data, 'Device updated'),
        error: (e) => this.fail(e),
      });
    } else {
      const body: CreateDeviceRequest = { name: this.form.name.trim(), mode: this.form.mode };
      if (this.form.serialNumber.trim()) body.serialNumber = this.form.serialNumber.trim();
      if (this.form.ipAddress.trim()) body.ipAddress = this.form.ipAddress.trim();
      if (Number(this.form.port)) body.port = Number(this.form.port);
      if (Number(this.form.commPassword)) body.commPassword = Number(this.form.commPassword);
      if (this.form.timezone.trim()) body.timezone = this.form.timezone.trim();
      if (Number(this.form.pollIntervalSeconds))
        body.pollIntervalSeconds = Number(this.form.pollIntervalSeconds);
      if (this.form.punchStateMap.trim()) body.punchStateMap = this.form.punchStateMap.trim();
      if (this.form.hrDeviceId.trim()) body.hrDeviceId = this.form.hrDeviceId.trim();
      if (this.form.hrDeviceKey) body.hrDeviceKey = this.form.hrDeviceKey;
      this.api.createDevice(body).subscribe({
        next: (r) => this.done(r.data, 'Device registered'),
        error: (e) => this.fail(e),
      });
    }
  }

  private done(dev: Device, msg: string): void {
    this.saving.set(false);
    this.toast.success(msg);
    this.saved.emit(dev);
  }

  private fail(e: unknown): void {
    this.saving.set(false);
    this.toast.error(apiError(e));
  }
}
