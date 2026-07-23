import { Injectable, computed, signal } from '@angular/core';

export type Theme = 'light' | 'dark';

const BASE_KEY = 'zkbio.baseUrl';
const API_KEY = 'zkbio.apiKey';
const THEME_KEY = 'zkbio.theme';

/** First-run guess for the gateway URL: the same host the console is served
 *  from, on the gateway's port 8080. When the console is served from the Pi
 *  (nginx :8082), this targets the gateway on that same Pi automatically; under
 *  `ng serve` on localhost it targets localhost:8080. Overridable in Settings. */
function defaultBaseUrl(): string {
  const { protocol, hostname } = window.location;
  const host = hostname || 'localhost';
  return `${protocol === 'https:' ? 'https:' : 'http:'}//${host}:8080`;
}

/** Persists the gateway connection settings (base URL + management API key) and
 *  the UI theme in localStorage. The base URL points at the Go gateway, e.g.
 *  http://raspberrypi.local:8080. The API key is sent as X-API-Key. */
@Injectable({ providedIn: 'root' })
export class ConfigService {
  readonly baseUrl = signal<string>(localStorage.getItem(BASE_KEY) ?? defaultBaseUrl());
  readonly apiKey = signal<string>(localStorage.getItem(API_KEY) ?? '');
  readonly theme = signal<Theme>((localStorage.getItem(THEME_KEY) as Theme) ?? 'light');

  readonly configured = computed(() => this.baseUrl().trim().length > 0);

  constructor() {
    this.applyTheme();
  }

  setBaseUrl(value: string): void {
    const clean = value.trim().replace(/\/+$/, '');
    this.baseUrl.set(clean);
    localStorage.setItem(BASE_KEY, clean);
  }

  setApiKey(value: string): void {
    const clean = value.trim();
    this.apiKey.set(clean);
    localStorage.setItem(API_KEY, clean);
  }

  setTheme(theme: Theme): void {
    this.theme.set(theme);
    localStorage.setItem(THEME_KEY, theme);
    this.applyTheme();
  }

  toggleTheme(): void {
    this.setTheme(this.theme() === 'dark' ? 'light' : 'dark');
  }

  private applyTheme(): void {
    document.documentElement.setAttribute('data-theme', this.theme());
  }
}
