const BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json();
}

export interface Room {
  id: number;
  name: string;
}

export interface QuestionSet {
  id: number;
  room_id: number;
  name: string;
  questions: Question[];
}

export interface Question {
  id: number;
  question_set_id: number;
  text: string;
  sort_order: number;
}

export interface SessionInfo {
  session_id: number;
  questions: { id: number; text: string; sort_order: number }[];
}

export interface VoteStatusResponse {
  status: string;
  participants: {
    participant_id: number;
    participant_name: string;
    has_voted: boolean;
  }[];
}

export interface ResultEntry {
  participant_id: number;
  participant_name: string;
  answers: Record<string, string>;
}

export const api = {
  listRooms: () => request<Room[]>('/rooms'),
  createRoom: (name: string, passphrase: string) =>
    request<{ id: number; name: string }>('/rooms', {
      method: 'POST',
      body: JSON.stringify({ name, passphrase }),
    }),
  login: (room_id: number, passphrase: string, name: string) =>
    request<{ room_id: number; room_name: string; participant_id: number }>(
      '/rooms/login',
      { method: 'POST', body: JSON.stringify({ room_id, passphrase, name }) }
    ),
  getParticipants: (roomId: number) =>
    request<{ id: number; name: string }[]>(`/rooms/${roomId}/participants`),
  listQuestionSets: (roomId: number) =>
    request<QuestionSet[]>(`/rooms/${roomId}/question-sets`),
  createQuestionSet: (
    roomId: number,
    name: string,
    questions: { text: string; sort_order: number }[]
  ) =>
    request<{ id: number }>(`/rooms/${roomId}/question-sets`, {
      method: 'POST',
      body: JSON.stringify({ name, questions }),
    }),
  updateQuestionSet: (
    qsId: number,
    name: string,
    questions: { text: string; sort_order: number }[]
  ) =>
    request<{ status: string }>(`/question-sets/${qsId}`, {
      method: 'PUT',
      body: JSON.stringify({ name, questions }),
    }),
  deleteQuestionSet: (qsId: number) =>
    request<{ status: string }>(`/question-sets/${qsId}`, { method: 'DELETE' }),
  startSession: (roomId: number, questionSetId: number) =>
    request<SessionInfo>(`/rooms/${roomId}/sessions`, {
      method: 'POST',
      body: JSON.stringify({ question_set_id: questionSetId }),
    }),
  submitAnswers: (
    sessionId: number,
    participantId: number,
    answers: Record<string, string>
  ) =>
    request<{ status: string; voted: number; total: number; all_voted: boolean }>(
      `/sessions/${sessionId}/answers`,
      {
        method: 'POST',
        body: JSON.stringify({ participant_id: participantId, answers }),
      }
    ),
  revealResults: (sessionId: number) =>
    request<{ status: string }>(`/sessions/${sessionId}/reveal`, {
      method: 'POST',
    }),
  resetSession: (sessionId: number) =>
    request<{ status: string }>(`/sessions/${sessionId}/reset`, {
      method: 'POST',
    }),
  getSessionQuestions: (sessionId: number) =>
    request<{ id: number; text: string; sort_order: number }[]>(
      `/sessions/${sessionId}/questions`
    ),
  getVoteStatus: (sessionId: number) =>
    request<VoteStatusResponse>(`/sessions/${sessionId}/status`),
  getResults: (sessionId: number) =>
    request<ResultEntry[]>(`/sessions/${sessionId}/results`),
};
