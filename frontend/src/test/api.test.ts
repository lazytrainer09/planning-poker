import { describe, it, expect, vi, beforeEach } from 'vitest'
import { api } from '../api'

// Mock fetch globally
const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

function mockResponse(data: unknown, ok = true, status = 200) {
  return {
    ok,
    status,
    statusText: ok ? 'OK' : 'Error',
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  }
}

beforeEach(() => {
  mockFetch.mockReset()
})

describe('api', () => {
  describe('listRooms', () => {
    it('fetches rooms from /api/rooms', async () => {
      const rooms = [{ id: 1, name: 'Room1' }]
      mockFetch.mockResolvedValue(mockResponse(rooms))

      const result = await api.listRooms()
      expect(result).toEqual(rooms)
      expect(mockFetch).toHaveBeenCalledWith('/api/rooms', expect.objectContaining({
        headers: { 'Content-Type': 'application/json' },
      }))
    })
  })

  describe('createRoom', () => {
    it('sends POST with name and passphrase', async () => {
      mockFetch.mockResolvedValue(mockResponse({ id: 1, name: 'New' }))

      const result = await api.createRoom('New', 'pass')
      expect(result).toEqual({ id: 1, name: 'New' })
      expect(mockFetch).toHaveBeenCalledWith('/api/rooms', expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'New', passphrase: 'pass' }),
      }))
    })
  })

  describe('login', () => {
    it('sends room_id when provided', async () => {
      mockFetch.mockResolvedValue(mockResponse({
        room_id: 1, room_name: 'Room', participant_id: 10,
      }))

      await api.login(1, '', 'pass', 'Alice')
      const body = JSON.parse(mockFetch.mock.calls[0][1].body)
      expect(body.room_id).toBe(1)
      expect(body.room_name).toBeUndefined()
    })

    it('sends room_name when room_id is 0', async () => {
      mockFetch.mockResolvedValue(mockResponse({
        room_id: 1, room_name: 'MyRoom', participant_id: 10,
      }))

      await api.login(0, 'MyRoom', 'pass', 'Bob')
      const body = JSON.parse(mockFetch.mock.calls[0][1].body)
      expect(body.room_id).toBeUndefined()
      expect(body.room_name).toBe('MyRoom')
    })
  })

  describe('createQuestionSet', () => {
    it('sends questions with name', async () => {
      mockFetch.mockResolvedValue(mockResponse({ id: 5 }))

      const questions = [{ text: 'Q1', sort_order: 0 }]
      await api.createQuestionSet(1, 'Sprint', questions)
      const body = JSON.parse(mockFetch.mock.calls[0][1].body)
      expect(body.name).toBe('Sprint')
      expect(body.questions).toEqual(questions)
    })
  })

  describe('updateQuestionSet', () => {
    it('sends PUT to correct endpoint', async () => {
      mockFetch.mockResolvedValue(mockResponse({ status: 'ok' }))

      await api.updateQuestionSet(5, 'Updated', [{ text: 'Q1', sort_order: 0 }])
      expect(mockFetch).toHaveBeenCalledWith('/api/question-sets/5', expect.objectContaining({
        method: 'PUT',
      }))
    })
  })

  describe('submitAnswers', () => {
    it('sends participant_id and answers', async () => {
      mockFetch.mockResolvedValue(mockResponse({
        status: 'ok', voted: 1, total: 2, all_voted: false,
      }))

      const result = await api.submitAnswers(1, 10, { '1': '3', '2': '5' })
      expect(result.all_voted).toBe(false)
      const body = JSON.parse(mockFetch.mock.calls[0][1].body)
      expect(body.participant_id).toBe(10)
      expect(body.answers).toEqual({ '1': '3', '2': '5' })
    })
  })

  describe('error handling', () => {
    it('throws on non-ok response', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        text: () => Promise.resolve('invalid passphrase'),
      })

      await expect(api.login(1, '', 'wrong', 'Eve')).rejects.toThrow('invalid passphrase')
    })
  })
})
