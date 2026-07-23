import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ConfigService } from './core/config.service';
import { ToastHost } from './core/toast-host';

@Component({
  selector: 'app-root',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, ToastHost],
  template: `
    <div class="shell">
      <aside class="sidebar">
        <div class="brand">
          <span class="logo">ZK</span>
          <div>
            <div class="brand-name">ZKBio Gateway</div>
            <div class="brand-sub">Attendance console</div>
          </div>
        </div>

        <nav class="nav">
          <a routerLink="/dashboard" routerLinkActive="active">
            <span class="ico">▤</span> Dashboard
          </a>
          <a routerLink="/devices" routerLinkActive="active">
            <span class="ico">▣</span> Devices
          </a>
          <a routerLink="/logs" routerLinkActive="active">
            <span class="ico">≡</span> Logs
          </a>
          <a routerLink="/settings" routerLinkActive="active">
            <span class="ico">⚙</span> Settings
          </a>
        </nav>

        <div class="side-foot">
          <div class="conn">
            <span class="dot" [class.ok]="cfg.apiKey()" [class.warn]="!cfg.apiKey()"></span>
            <span class="conn-url" [title]="cfg.baseUrl()">{{ cfg.baseUrl() }}</span>
          </div>
          <button class="btn ghost sm theme-btn" (click)="cfg.toggleTheme()">
            {{ cfg.theme() === 'dark' ? '☀ Light' : '☾ Dark' }}
          </button>
        </div>
      </aside>

      <main class="main">
        <router-outlet />
      </main>
    </div>

    <app-toast-host />
  `,
  styles: [
    `
      .shell { display: flex; min-height: 100vh; }
      .sidebar {
        width: var(--sidebar-w); flex-shrink: 0; background: var(--surface);
        border-right: 1px solid var(--border); display: flex; flex-direction: column;
        position: sticky; top: 0; height: 100vh;
      }
      .brand { display: flex; align-items: center; gap: 11px; padding: 20px 18px; border-bottom: 1px solid var(--border); }
      .logo {
        width: 38px; height: 38px; border-radius: 9px; background: var(--primary); color: #fff;
        display: grid; place-items: center; font-weight: 800; font-size: 0.9rem; flex-shrink: 0;
      }
      .brand-name { font-weight: 700; font-size: 0.95rem; }
      .brand-sub { color: var(--text-faint); font-size: 0.76rem; }

      .nav { display: flex; flex-direction: column; gap: 2px; padding: 14px 12px; flex: 1; }
      .nav a {
        display: flex; align-items: center; gap: 11px; padding: 9px 12px; border-radius: var(--radius-sm);
        color: var(--text-muted); font-weight: 600; font-size: 0.9rem; text-decoration: none;
      }
      .nav a:hover { background: var(--surface-2); color: var(--text); text-decoration: none; }
      .nav a.active { background: var(--primary-soft); color: var(--primary); }
      .nav .ico { width: 18px; text-align: center; opacity: 0.85; }

      .side-foot { padding: 14px 16px; border-top: 1px solid var(--border); display: flex; flex-direction: column; gap: 10px; }
      .conn { display: flex; align-items: center; gap: 8px; font-size: 0.76rem; color: var(--text-muted); }
      .conn-url { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .theme-btn { align-self: flex-start; }

      .main { flex: 1; min-width: 0; }

      @media (max-width: 720px) {
        .shell { flex-direction: column; }
        .sidebar { width: 100%; height: auto; position: static; flex-direction: row; flex-wrap: wrap; align-items: center; }
        .brand { border-bottom: none; }
        .nav { flex-direction: row; flex: 1 1 100%; overflow-x: auto; padding: 0 12px 12px; }
        .side-foot { border-top: none; flex-direction: row; align-items: center; }
      }
    `,
  ],
})
export class App {
  protected cfg = inject(ConfigService);
}
