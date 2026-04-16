import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, QuestionSet } from '../api'
import { connectWS } from '../ws'

export default function RoomPage() {
  const { roomId } = useParams<{ roomId: string }>()
  const navigate = useNavigate()
  const rid = Number(roomId)
  const participantId = Number(sessionStorage.getItem('participant_id'))
  const roomName = sessionStorage.getItem('room_name') || `Room ${rid}`

  const [questionSets, setQuestionSets] = useState<QuestionSet[]>([])
  const [participants, setParticipants] = useState<{ id: number; name: string }[]>([])
  const [validated, setValidated] = useState(false)

  // Validate participant on mount (handles page reload)
  useEffect(() => {
    if (!participantId) {
      navigate('/')
      return
    }
    api.validateParticipant(rid, participantId)
      .then((res) => {
        sessionStorage.setItem('room_name', res.room_name)
        setValidated(true)
      })
      .catch(() => {
        sessionStorage.clear()
        navigate('/')
      })
  }, [rid, participantId, navigate])

  const loadData = useCallback(async () => {
    const [qs, ps] = await Promise.all([
      api.listQuestionSets(rid),
      api.getParticipants(rid),
    ])
    setQuestionSets(qs)
    setParticipants(ps)
  }, [rid])

  useEffect(() => {
    if (validated) loadData()
  }, [loadData, validated])

  useEffect(() => {
    const disconnect = connectWS(rid, participantId, (msg) => {
      if (msg.type === 'participant_joined' || msg.type === 'participant_left') {
        api.getParticipants(rid).then(setParticipants)
      }
      if (msg.type === 'session_started') {
        sessionStorage.setItem(
          `session_${msg.payload.session_id}_questions`,
          JSON.stringify(msg.payload.questions)
        )
        navigate(`/room/${rid}/vote/${msg.payload.session_id}`)
      }
    })
    return disconnect
  }, [rid, participantId, navigate])

  const handleStartVoting = async (qsId: number) => {
    const res = await api.startSession(rid, qsId)
    sessionStorage.setItem(`session_${res.session_id}_questions`, JSON.stringify(res.questions))
    navigate(`/room/${rid}/vote/${res.session_id}`)
  }

  const handleDelete = async (qsId: number) => {
    if (!confirm('この質問セットを削除しますか？')) return
    await api.deleteQuestionSet(qsId)
    loadData()
  }

  return (
    <>
      <div className="header">
        <h1>{roomName}</h1>
        <Link to="/" className="back-link">退室</Link>
      </div>

      <div className="card">
        <h2>参加者</h2>
        <div className="participants-list">
          {participants.map((p) => (
            <span key={p.id} className="participant-badge">
              {p.name}
            </span>
          ))}
          {participants.length === 0 && (
            <span className="empty-state">参加者がいません</span>
          )}
        </div>
      </div>

      <div className="card">
        <div className="header" style={{ marginBottom: 12 }}>
          <h2 style={{ marginBottom: 0 }}>質問セット</h2>
          <button
            className="btn-primary btn-sm"
            onClick={() => navigate(`/room/${rid}/question-set`)}
          >
            + 新規
          </button>
        </div>

        {questionSets.length === 0 && (
          <p className="empty-state">質問セットがありません。新規作成して投票を始めましょう。</p>
        )}

        {questionSets.map((qs) => (
          <div key={qs.id} className="qs-item">
            <div>
              <strong>{qs.name}</strong>
              <span style={{ color: '#999', marginLeft: 8, fontSize: '0.85rem' }}>
                ({qs.questions.length}問)
              </span>
            </div>
            <div className="btn-group">
              <button
                className="btn-success btn-sm"
                onClick={() => handleStartVoting(qs.id)}
              >
                投票開始
              </button>
              <button
                className="btn-secondary btn-sm"
                onClick={() => navigate(`/room/${rid}/question-set/${qs.id}`)}
              >
                編集
              </button>
              <button
                className="btn-danger btn-sm"
                onClick={() => handleDelete(qs.id)}
              >
                削除
              </button>
            </div>
          </div>
        ))}
      </div>
    </>
  )
}
