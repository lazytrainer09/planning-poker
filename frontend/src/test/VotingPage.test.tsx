import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import VotingPage from '../pages/VotingPage'
import { api } from '../api'

vi.mock('../api', () => ({
  api: {
    listQuestionSets: vi.fn(),
    getSessionQuestions: vi.fn(),
    getVoteStatus: vi.fn(),
    getResults: vi.fn(),
    submitAnswers: vi.fn(),
    resetSession: vi.fn(),
    startSession: vi.fn(),
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

const mockQuestions = [
  { id: 1, text: 'Complexity', sort_order: 0 },
  { id: 2, text: 'Risk', sort_order: 1 },
]

function renderVoting(sessionId = '1') {
  sessionStorage.setItem('participant_id', '10')
  return render(
    <MemoryRouter initialEntries={[`/room/1/vote/${sessionId}`]}>
      <Routes>
        <Route path="/room/:roomId/vote/:sessionId" element={<VotingPage />} />
      </Routes>
    </MemoryRouter>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  vi.mocked(api.listQuestionSets).mockResolvedValue([])
  vi.mocked(api.getSessionQuestions).mockResolvedValue(mockQuestions)
  vi.mocked(api.getVoteStatus).mockResolvedValue({
    status: 'voting',
    question_set_id: 1,
    participants: [
      { participant_id: 10, participant_name: 'Alice', has_voted: false },
    ],
  })
})

describe('VotingPage', () => {
  it('loads and displays questions', async () => {
    renderVoting()

    await waitFor(() => {
      expect(screen.getByText('Complexity')).toBeInTheDocument()
      expect(screen.getByText('Risk')).toBeInTheDocument()
    })
  })

  it('shows participant vote status', async () => {
    renderVoting()

    await waitFor(() => {
      expect(screen.getByText(/Alice/)).toBeInTheDocument()
      expect(screen.getByText(/未回答/)).toBeInTheDocument()
    })
  })

  it('submits answers', async () => {
    vi.mocked(api.submitAnswers).mockResolvedValue({
      status: 'ok', voted: 1, total: 2, all_voted: false,
    })
    renderVoting()
    const user = userEvent.setup()

    await waitFor(() => {
      expect(screen.getByText('Complexity')).toBeInTheDocument()
    })

    const inputs = screen.getAllByPlaceholderText('回答を入力...')
    await user.type(inputs[0], '3')
    await user.type(inputs[1], '5')
    await user.click(screen.getByText('回答を送信'))

    await waitFor(() => {
      expect(api.submitAnswers).toHaveBeenCalledWith(1, 10, { '1': '3', '2': '5' })
      expect(screen.getByText(/他のメンバーの回答を待っています/)).toBeInTheDocument()
    })
  })

  it('shows results when revealed', async () => {
    vi.mocked(api.getVoteStatus).mockResolvedValue({
      status: 'revealed',
      question_set_id: 1,
      participants: [
        { participant_id: 10, participant_name: 'Alice', has_voted: true },
      ],
    })
    vi.mocked(api.getResults).mockResolvedValue([
      {
        participant_id: 10,
        participant_name: 'Alice',
        answers: { '1': '3', '2': '5' },
      },
    ])
    renderVoting()

    await waitFor(() => {
      expect(screen.getByText('結果')).toBeInTheDocument()
      expect(screen.getByText('3')).toBeInTheDocument()
      expect(screen.getByText('5')).toBeInTheDocument()
    })
  })

  it('shows loading state when no questions', async () => {
    vi.mocked(api.getSessionQuestions).mockResolvedValue([])
    renderVoting()

    await waitFor(() => {
      expect(screen.getByText(/質問を読み込み中/)).toBeInTheDocument()
    })
  })

  it('disables inputs after submission', async () => {
    vi.mocked(api.submitAnswers).mockResolvedValue({
      status: 'ok', voted: 1, total: 2, all_voted: false,
    })
    renderVoting()
    const user = userEvent.setup()

    await waitFor(() => {
      expect(screen.getByText('Complexity')).toBeInTheDocument()
    })

    await user.click(screen.getByText('回答を送信'))

    await waitFor(() => {
      const inputs = screen.getAllByPlaceholderText('回答を入力...')
      inputs.forEach((input) => {
        expect(input).toBeDisabled()
      })
    })
  })
})
