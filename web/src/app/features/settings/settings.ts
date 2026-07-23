import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService, apiError } from '../../core/api.service';
import { ConfigService } from '../../core/config.service';
import { ToastService } from '../../core/toast.service';

@Component({
  selector: 'app-settings',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  template: `
    <div class="page">
      <div class="page-head">
        <div>
          <h1>Settings</h1>
          <p class="subtitle">How this console reaches the gateway. Stored only in this browser.</p>
        </div>
      </div>

      <div class="card card-pad" style="max-width:620px">
        <div class="field">
          <label>Gateway base URL</label>
          <input [(ngModel)]="baseUrl" class="mono" placeholder="http://raspberrypi.local:8080" />
          <div class="hint">The address of the Go gateway. No trailing slash needed.</div>
        </div>

        <div class="field">
          <label>Management API key</label>
          <div class="row" style="flex-wrap:nowrap">
            <input [type]="reveal() ? 'text' : 'password'" [(ngModel)]="apiKey" class="mono grow"
                   placeholder="MGMT_API_KEY (leave blank if gateway runs open)" />
            <button class="btn" type="button" (click)="reveal.set(!reveal())">
              {{ reveal() ? 'Hide' : 'Show' }}
            </button>
          </div>
          <div class="hint">Sent as <span class="mono">X-API-Key</span>. Leave empty if MGMT_API_KEY is unset on the gateway.</div>
        </div>

        <div class="field">
          <label>Theme</label>
          <select [ngModel]="cfg.theme()" (ngModelChange)="cfg.setTheme($event)">
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </div>

        <div class="row" style="margin-top:8px">
          <button class="btn primary" (click)="save()">Save</button>
          <button class="btn" (click)="test()" [disabled]="testing()">
            @if (testing()) { <span class="spinner"></span> } Test connection
          </button>
          @if (result(); as r) {
            <span class="badge" [class.ok]="r.ok" [class.danger]="!r.ok">{{ r.msg }}</span>
          }
        </div>
      </div>
    </div>
  `,
})
export class SettingsPage {
  protected cfg = inject(ConfigService);
  private api = inject(ApiService);
  private toast = inject(ToastService);

  protected baseUrl = this.cfg.baseUrl();
  protected apiKey = this.cfg.apiKey();
  protected reveal = signal(false);
  protected testing = signal(false);
  protected result = signal<{ ok: boolean; msg: string } | null>(null);

  save(): void {
    this.cfg.setBaseUrl(this.baseUrl);
    this.cfg.setApiKey(this.apiKey);
    this.baseUrl = this.cfg.baseUrl();
    this.result.set(null);
    this.toast.success('Settings saved');
  }

  test(): void {
    // Persist first so the request uses the values on screen.
    this.cfg.setBaseUrl(this.baseUrl);
    this.cfg.setApiKey(this.apiKey);
    this.baseUrl = this.cfg.baseUrl();
    this.testing.set(true);
    this.result.set(null);
    this.api.getDashboard().subscribe({
      next: (d) => {
        this.testing.set(false);
        this.result.set({ ok: true, msg: `Connected · ${d.devices} device(s)` });
      },
      error: (e) => {
        this.testing.set(false);
        this.result.set({ ok: false, msg: apiError(e) });
      },
    });
  }
}
