import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { ConfigService } from './config.service';

/** Attaches the management API key (X-API-Key) to any request aimed at the
 *  configured gateway base URL. The gateway also accepts Authorization: Bearer,
 *  but X-API-Key is explicitly CORS-allow-listed, so we use it. */
export const apiKeyInterceptor: HttpInterceptorFn = (req, next) => {
  const cfg = inject(ConfigService);
  const base = cfg.baseUrl();
  const key = cfg.apiKey();
  if (base && key && req.url.startsWith(base)) {
    req = req.clone({ setHeaders: { 'X-API-Key': key } });
  }
  return next(req);
};
