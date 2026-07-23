import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { ToastService } from './toast.service';

@Component({
  selector: 'app-toast-host',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="toast-host">
      @for (t of toast.toasts(); track t.id) {
        <div class="toast" [class.success]="t.kind === 'success'"
             [class.error]="t.kind === 'error'" [class.info]="t.kind === 'info'">
          <span>{{ t.message }}</span>
          <button class="close" (click)="toast.dismiss(t.id)" aria-label="Dismiss">×</button>
        </div>
      }
    </div>
  `,
})
export class ToastHost {
  protected toast = inject(ToastService);
}
