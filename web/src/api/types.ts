// API types mirroring the Go backend JSON responses.

export type Role = 'customer' | 'agent' | 'admin';
export type TicketStatus = 'open' | 'pending' | 'solved' | 'closed';

export interface User {
  id: string;
  email: string;
  fullName: string;
  role: Role;
  createdAt: string;
}

export interface UserRef {
  id: string;
  name: string;
}

export interface Ticket {
  id: string;
  ticketNumber: string;
  subject: string;
  description: string;
  type: string;
  severity: string;
  status: TicketStatus;
  requester: UserRef;
  assignee: UserRef | null;
  notifyEmails: string[];
  createdAt: string;
  updatedAt: string;
  solvedAt: string | null;
}

export interface LinkedTicket {
  id: string;
  ticketNumber: string;
  subject: string;
  status: TicketStatus;
  relation: string;
  createdAt: string;
}

export interface Attachment {
  id: string;
  ticketId: string;
  uploader: UserRef;
  fileName: string;
  contentType: string;
  sizeBytes: number;
  createdAt: string;
}

export interface TicketDetail extends Ticket {
  linkedTickets: LinkedTicket[];
  attachments: Attachment[];
}

export interface Message {
  id: string;
  ticketId: string;
  author: UserRef;
  authorRole: Role;
  body: string;
  isInternal: boolean;
  createdAt: string;
}

export interface TicketEvent {
  id: number;
  ticketId: string;
  actor: UserRef | null;
  type: 'created' | 'assigned' | 'status_changed' | 'message_added' | 'linked';
  data: Record<string, unknown>;
  createdAt: string;
}

export interface TimelineItem {
  kind: 'message' | 'event';
  createdAt: string;
  message?: Message;
  event?: TicketEvent;
}

export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface Enums {
  types: string[];
  severities: string[];
  statuses: TicketStatus[];
}

export interface Summary {
  total: number;
  byStatus: Record<string, number>;
  bySeverity: Record<string, number>;
  byType: Record<string, number>;
  recent: Ticket[];
}

export interface TicketFilters {
  status?: string;
  type?: string;
  severity?: string;
  search?: string;
  sort?: string;
  page?: number;
  pageSize?: number;
}

export interface AuthResponse {
  accessToken: string;
  user: User;
}
