import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'dashboard' },
  {
    path: 'dashboard',
    title: 'Dashboard · ZKBio',
    loadComponent: () => import('./features/dashboard/dashboard').then((m) => m.DashboardPage),
  },
  {
    path: 'devices',
    title: 'Devices · ZKBio',
    loadComponent: () => import('./features/devices/device-list').then((m) => m.DeviceListPage),
  },
  {
    path: 'devices/:id',
    title: 'Device · ZKBio',
    loadComponent: () => import('./features/devices/device-detail').then((m) => m.DeviceDetailPage),
  },
  {
    path: 'logs',
    title: 'Logs · ZKBio',
    loadComponent: () => import('./features/logs/logs').then((m) => m.LogsPage),
  },
  {
    path: 'settings',
    title: 'Settings · ZKBio',
    loadComponent: () => import('./features/settings/settings').then((m) => m.SettingsPage),
  },
  { path: '**', redirectTo: 'dashboard' },
];
