import { api } from './client';
import type {
  Attachment, AuthResponse, Enums, Message, Page, Summary, Ticket, TicketDetail, TicketFilters, TimelineItem, User,
} from './types';

// ---------- Auth ----------
export const authApi = {
  login: (email: string, password: string) =>
    api.post<AuthResponse>('/auth/login', { email, password }).then((r) => r.data),
  register: (fullName: string, email: string, password: string) =>
    api.post<User>('/auth/register', { fullName, email, password }).then((r) => r.data),
  logout: () => api.post('/auth/logout'),
};

// ---------- Tickets ----------
export interface NewTicketInput {
  subject: string;
  description: string;
  type: string;
  severity: string;
  linkedTicketIds: string[];
  /** ";"-separated list of emails to notify. */
  notifyEmails: string;
}

/** Drops empty filter values so the URL stays clean. */
function cleanParams(filters: TicketFilters) {
  return Object.fromEntries(Object.entries(filters).filter(([, v]) => v !== '' && v != null));
}

export const ticketsApi = {
  list: (filters: TicketFilters) =>
    api.get<Page<Ticket>>('/tickets', { params: cleanParams(filters) }).then((r) => r.data),
  get: (id: string) => api.get<TicketDetail>(`/tickets/${id}`).then((r) => r.data),
  timeline: (id: string) => api.get<TimelineItem[]>(`/tickets/${id}/timeline`).then((r) => r.data),
  create: (input: NewTicketInput) => api.post<TicketDetail>('/tickets', input).then((r) => r.data),
  addMessage: (id: string, body: string, isInternal: boolean) =>
    api.post<Message>(`/tickets/${id}/messages`, { body, isInternal }).then((r) => r.data),
  changeStatus: (id: string, status: string) =>
    api.patch<TicketDetail>(`/tickets/${id}/status`, { status }).then((r) => r.data),
  assign: (id: string, assigneeId: string) =>
    api.patch<TicketDetail>(`/tickets/${id}/assignee`, { assigneeId }).then((r) => r.data),
  addLink: (id: string, linkedTicketId: string) =>
    api.post<TicketDetail>(`/tickets/${id}/links`, { linkedTicketId }).then((r) => r.data),
  removeLink: (id: string, linkedId: string) => api.delete(`/tickets/${id}/links/${linkedId}`),
  uploadAttachment: (id: string, file: File) => {
    const form = new FormData();
    form.append('file', file);
    return api.post<Attachment>(`/tickets/${id}/attachments`, form).then((r) => r.data);
  },
  /** Downloads with the auth header, then triggers a browser save. */
  downloadAttachment: async (id: string, attachment: Attachment) => {
    const res = await api.get<Blob>(`/tickets/${id}/attachments/${attachment.id}`, { responseType: 'blob' });
    const url = URL.createObjectURL(res.data);
    const link = document.createElement('a');
    link.href = url;
    link.download = attachment.fileName;
    document.body.appendChild(link); // Firefox/Safari need the link in the DOM
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000); // revoking immediately can cancel the download
  },
};

// ---------- Meta, reports, agents ----------
export const metaApi = {
  enums: () => api.get<Enums>('/meta/enums').then((r) => r.data),
  summary: () => api.get<Summary>('/reports/summary').then((r) => r.data),
  agents: () => api.get<User[]>('/agents').then((r) => r.data),
  createAgent: (fullName: string, email: string, password: string) =>
    api.post<User>('/agents', { fullName, email, password }).then((r) => r.data),
};
