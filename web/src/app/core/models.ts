// TypeScript mirror of the gateway's management API contract
// (ismila/erp/zkbio — mila_structs/*). Keep in lock-step with the Go structs.

export type State = 'success' | 'failed' | 'error';

export interface AnswerResponse<T> {
  state: State;
  message: string;
  isSuccess: boolean;
  data: T;
  adv: string;
}

export interface Pagination {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
}

export interface PaginationResponse<T> {
  state: State;
  data: T[];
  message: string;
  pagination: Pagination;
}

export type DeviceMode = 'pull' | 'push' | 'both';
export type DeliveryStatus = 'Pending' | 'Sent' | 'Failed';
export type LogType = 'IN' | 'OUT';

export interface Device {
  id: string;
  name: string;
  serialNumber: string;
  ipAddress: string;
  port: number;
  mode: DeviceMode;
  timezone: string;
  hrDeviceId: string;
  hrKeyPrefix: string;
  hrCredentialSet: boolean;
  punchStateMap: string;
  isActive: boolean;
  pollIntervalSeconds: number;
  lastSeenAt: string | null;
  lastPullAt: string | null;
  lastPullError: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateDeviceRequest {
  name: string;
  serialNumber?: string;
  ipAddress?: string;
  port?: number;
  commPassword?: number;
  mode?: DeviceMode;
  timezone?: string;
  hrDeviceId?: string;
  hrDeviceKey?: string;
  punchStateMap?: string;
  pollIntervalSeconds?: number;
}

export interface UpdateDeviceRequest {
  name?: string;
  serialNumber?: string;
  ipAddress?: string;
  port?: number;
  commPassword?: number;
  mode?: DeviceMode;
  timezone?: string;
  punchStateMap?: string;
  pollIntervalSeconds?: number;
  isActive?: boolean;
}

export interface SetHRCredentialRequest {
  hrDeviceId: string;
  hrDeviceKey: string;
}

export interface Punch {
  id: string;
  deviceId: string | null;
  deviceSerial: string;
  employeeNumber: string;
  punchState: number;
  logType: LogType;
  eventTime: string;
  verifyMode: number;
  source: string;
  deliveryStatus: DeliveryStatus;
  deliveryAttempts: number;
  lastError: string;
  sentAt: string | null;
  createdAt: string;
}

export interface DeviceHealth {
  id: string;
  name: string;
  serialNumber: string;
  mode: string;
  isActive: boolean;
  hrCredentialSet: boolean;
  lastSeenAt: string | null;
  pending: number;
  failed: number;
  online: boolean;
}

export interface Dashboard {
  devices: number;
  activeDevices: number;
  pendingPunches: number;
  failedPunches: number;
  sentPunches: number;
  totalPunches: number;
  oldestPending: string | null;
  lastDelivery: string | null;
  deviceHealth: DeviceHealth[];
}

export interface PunchFilter {
  page?: number;
  pageSize?: number;
  deviceId?: string;
  employeeNumber?: string;
  status?: DeliveryStatus | '';
  from?: string;
  to?: string;
}
