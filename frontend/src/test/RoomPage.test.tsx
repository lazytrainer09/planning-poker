import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import RoomPage from '../pages/RoomPage'
import { api } from '../api'

vi.mock('../api', () => ({
  api: {
    validateParticipant: vi.fn(),
    listQuestionSets: vi.fn(),
    getParticipants: vi.fn(),
    startSession: vi.fn(),
    deleteQuestionSet: vi.fn(),
  },
}))

vi.mock('../ws', () => ({
  connectWS: vi.fn(() => () => {}),
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

function renderRoom() {
  sessionStorage.setItem('participant_id', '1')
  sessionStorage.setItem('room_name', 'TestRoom')
  return render(
    <MemoryRouter initialEntries={['/room/1']}>
      <Routes>
        <Route path="/room/:roomId" element={<RoomPage />} />
      </Routes>
    </MemoryRouter>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  vi.mocked(api.validateParticipant).mockResolvedValue({
    valid: true, name: 'Tester', room_name: 'TestRoom',
  })
  vi.mocked(api.listQuestionSets).mockResolvedValue([])
  vi.mocked(api.getParticipants).mockResolvedValue([])
})

describe('RoomPage', () => {
  it('redirects to top if no participant_id', async () => {
    // Don't set participant_id
    render(
      <MemoryRouter initialEntries={['/room/1']}>
        <Routes>
          <Route path="/room/:roomId" element={<RoomPage />} />
        </Routes>
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })

  it('validates participant and shows room name', async () => {
    renderRoom()

    await waitFor(() => {
      expect(api.validateParticipant).toHaveBeenCalledWith(1, 1)
      expect(screen.getByText('TestRoom')).toBeInTheDocument()
    })
  })

  it('shows participants', async () => {
    vi.mocked(api.getParticipants).mockResolvedValue([
      { id: 1, name: 'Alice' },
      { id: 2, name: 'Bob' },
    ])
    renderRoom()

    await waitFor(() => {
      expect(screen.getByText('Alice')).toBeInTheDocument()
      expect(screen.getByText('Bob')).toBeInTheDocument()
    })
  })

  it('shows question sets with controls', async () => {
    vi.mocked(api.listQuestionSets).mockResolvedValue([
      {
        id: 1, room_id: 1, name: 'Sprint 1',
        questions: [
          { id: 1, question_set_id: 1, text: 'Q1', sort_order: 0 },
          { id: 2, question_set_id: 1, text: 'Q2', sort_order: 1 },
        ],
      },
    ])
    renderRoom()

    await waitFor(() => {
      expect(screen.getByText('Sprint 1')).toBeInTheDocument()
      expect(screen.getByText('(2問)')).toBeInTheDocument()
      expect(screen.getByText('投票開始')).toBeInTheDocument()
      expect(screen.getByText('編集')).toBeInTheDocument()
      expect(screen.getByText('削除')).toBeInTheDocument()
    })
  })

  it('shows empty state when no question sets', async () => {
    renderRoom()

    await waitFor(() => {
      expect(screen.getByText(/質問セットがありません/)).toBeInTheDocument()
    })
  })

  it('redirects to top on validation failure', async () => {
    vi.mocked(api.validateParticipant).mockRejectedValue(new Error('invalid'))
    renderRoom()

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })
})
