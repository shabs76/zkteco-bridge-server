import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService, apiError } from '../../core/api.service';
import { ToastService } from '../../core/toast.service';
import { Device } from '../../core/models';

/** Modal to set or rotate a device's HR credential. The key is write-only —
 *  the gateway stores it encrypted and never returns it. */
@Component({
  selector: 'app-credential-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  template: `
    <div class="modal-backdrop" (click)="onBackdrop($event)">
      <div class="modal">
        <div class="modal-head">
          <h3>{{ device().hrCredentialSet ? 'Rotate' : 'Set' }} HR credential</h3>
          <button class="btn ghost icon" (click)="close.emit()" aria-label="Close">×</button>
        </div>
        <div class="modal-body">
          <p class="muted">
            For <strong>{{ device().name }}</strong>. Obtain these from the HR service
            (<span class="mono">POST /devices</span>). The key is encrypted at rest and never shown again.
          </p>
          <div class="field">
            <label>HR device ID *</label>
            <input [(ngModel)]="hrDeviceId" class="mono" placeholder="X-Device-Id" />
          </div>
          <div class="field">
            <label>HR device key *</label>
            <input [(ngModel)]="hrDeviceKey" type="password" class="mono" placeholder="X-Device-Key" />
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" (click)="close.emit()" [disabled]="saving()">Cancel</button>
          <button class="btn primary" (click)="save()" [disabled]="saving() || !valid()">
            @if (saving()) { <span class="spinner"></span> } Save credential
          </button>
        </div>
      </div>
    </div>
  `,
})
export class CredentialForm {
  private api = inject(ApiService);
  private toast = inject(ToastService);

  readonly device = input.required<Device>();
  readonly close = output<void>();
  readonly saved = output<Device>();

  protected hrDeviceId = '';
  protected hrDeviceKey = '';
  protected saving = signal(false);

  protected valid(): boolean {
    return this.hrDeviceId.trim().length > 0 && this.hrDeviceKey.length > 0;
  }

  onBackdrop(e: MouseEvent): void {
    if ((e.target as HTMLElement).classList.contains('modal-backdrop')) this.close.emit();
  }

  save(): void {
    this.saving.set(true);
    this.api
      .setCredential(this.device().id, {
        hrDeviceId: this.hrDeviceId.trim(),
        hrDeviceKey: this.hrDeviceKey,
      })
      .subscribe({
        next: (r) => {
          this.saving.set(false);
          this.toast.success('HR credential saved');
          this.saved.emit(r.data);
        },
        error: (e) => {
          this.saving.set(false);
          this.toast.error(apiError(e));
        },
      });
  }
}
