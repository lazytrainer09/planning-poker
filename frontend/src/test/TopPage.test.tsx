import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import TopPage from '../pages/TopPage'
import { api } from '../api'

// Mock api module
vi.mock('../api', () => ({
  api: {
    listRooms: vi.fn(),
    createRoom: vi.fn(),
    login: vi.fn(),
  },
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

function renderTop() {
  return render(
    <MemoryRouter>
      <TopPage />
    </MemoryRouter>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  vi.mocked(api.listRooms).mockResolvedValue([])
})

describe('TopPage', () => {
  it('renders title and sections', async () => {
    renderTop()
    expect(screen.getByText('プランニングポーカー')).toBeInTheDocument()
    expect(screen.getByText('ルーム作成')).toBeInTheDocument()
    expect(screen.getByText('ルームに参加')).toBeInTheDocument()
  })

  it('loads and displays rooms', async () => {
    vi.mocked(api.listRooms).mockResolvedValue([
      { id: 1, name: 'Sprint1' },
      { id: 2, name: 'Sprint2' },
    ])
    renderTop()

    await waitFor(() => {
      expect(screen.getByText('Sprint1')).toBeInTheDocument()
      expect(screen.getByText('Sprint2')).toBeInTheDocument()
    })
  })

  it('creates a room', async () => {
    vi.mocked(api.createRoom).mockResolvedValue({ id: 1, name: 'NewRoom' })
    renderTop()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('例: スプリントプランニング'), 'NewRoom')
    // Both sections have '合言葉を入力' — pick the first (create section)
    const passInputs = screen.getAllByPlaceholderText('合言葉を入力')
    await user.type(passInputs[0], 'secret')
    await user.click(screen.getByText('作成'))

    await waitFor(() => {
      expect(api.createRoom).toHaveBeenCalledWith('NewRoom', 'secret')
    })
  })

  it('logs in and navigates to room', async () => {
    vi.mocked(api.listRooms).mockResolvedValue([{ id: 1, name: 'Room1' }])
    vi.mocked(api.login).mockResolvedValue({
      room_id: 1, room_name: 'Room1', participant_id: 42,
    })
    renderTop()
    const user = userEvent.setup()

    // Wait for rooms to load
    await waitFor(() => expect(screen.getByText('Room1')).toBeInTheDocument())

    // Select room
    await user.selectOptions(screen.getByRole('combobox'), '1')

    // Fill in passphrase and name
    const passInputs = screen.getAllByPlaceholderText('合言葉を入力')
    await user.type(passInputs[passInputs.length - 1], 'pass')
    await user.type(screen.getByPlaceholderText('例: 太郎'), 'Alice')
    await user.click(screen.getByText('参加'))

    await waitFor(() => {
      expect(api.login).toHaveBeenCalledWith(1, '', 'pass', 'Alice')
      expect(mockNavigate).toHaveBeenCalledWith('/room/1')
      expect(sessionStorage.getItem('participant_id')).toBe('42')
    })
  })

  it('shows error on login failure', async () => {
    vi.mocked(api.listRooms).mockResolvedValue([{ id: 1, name: 'Room1' }])
    vi.mocked(api.login).mockRejectedValue(new Error('invalid passphrase'))
    renderTop()
    const user = userEvent.setup()

    await waitFor(() => expect(screen.getByText('Room1')).toBeInTheDocument())

    await user.selectOptions(screen.getByRole('combobox'), '1')
    const passInputs = screen.getAllByPlaceholderText('合言葉を入力')
    await user.type(passInputs[passInputs.length - 1], 'wrong')
    await user.type(screen.getByPlaceholderText('例: 太郎'), 'Eve')
    await user.click(screen.getByText('参加'))

    await waitFor(() => {
      expect(screen.getByText('invalid passphrase')).toBeInTheDocument()
    })
  })

  it('supports login by room name', async () => {
    vi.mocked(api.login).mockResolvedValue({
      room_id: 1, room_name: 'ByName', participant_id: 10,
    })
    renderTop()
    const user = userEvent.setup()

    // Switch to name mode
    await user.click(screen.getByText('ルーム名で入力'))
    await user.type(screen.getByPlaceholderText('ルーム名を入力'), 'ByName')
    const passInputs = screen.getAllByPlaceholderText('合言葉を入力')
    await user.type(passInputs[passInputs.length - 1], 'pass')
    await user.type(screen.getByPlaceholderText('例: 太郎'), 'Bob')
    await user.click(screen.getByText('参加'))

    await waitFor(() => {
      expect(api.login).toHaveBeenCalledWith(0, 'ByName', 'pass', 'Bob')
    })
  })
})
