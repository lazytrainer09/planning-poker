import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, ResultEntry } from '../api'
import { connectWS } from '../ws'

interface QuestionItem {
  id: number
  text: string
  sort_order: number
}

interface ParticipantStatus {
  participant_id: number
  participant_name: string
  has_voted: boolean
}

export default function VotingPage() {
  const { roomId, sessionId } = useParams<{ roomId: string; sessionId: string }>()
  const navigate = useNavigate()
  const rid = Number(roomId)
  const sid = Number(sessionId)
  const participantId = Number(sessionStorage.getItem('participant_id'))

  const [questions, setQuestions] = useState<QuestionItem[]>([])
  const [answers, setAnswers] = useState<Record<string, string>>({})
  const [submitted, setSubmitted] = useState(false)
  const [statuses, setStatuses] = useState<ParticipantStatus[]>([])
  const [revealed, setRevealed] = useState(false)
  const [results, setResults] = useState<ResultEntry[]>([])

  // Load questions: try sessionStorage first, then API fallback
  useEffect(() => {
    const stored = sessionStorage.getItem(`session_${sid}_questions`)
    if (stored) {
      setQuestions(JSON.parse(stored))
    } else {
      api.getSessionQuestions(sid).then((qs) => {
        setQuestions(qs)
        sessionStorage.setItem(`session_${sid}_questions`, JSON.stringify(qs))
      }).catch(() => {})
    }
  }, [sid])

  const loadStatus = useCallback(async () => {
    try {
      const data = await api.getVoteStatus(sid)
      setStatuses(data.participants)
      if (data.status === 'revealed') {
        setRevealed(true)
        const r = await api.getResults(sid)
        setResults(r)
      }
    } catch {
      // session may not exist yet
    }
  }, [sid])

  useEffect(() => {
    loadStatus()
  }, [loadStatus])

  useEffect(() => {
    const disconnect = connectWS(rid, participantId, (msg) => {
      if (msg.type === 'vote_submitted') {
        loadStatus()
      }
      if (msg.type === 'results_revealed') {
        setRevealed(true)
        setResults(msg.payload)
      }
      if (msg.type === 'session_reset') {
        setRevealed(false)
        setResults([])
        setSubmitted(false)
        setAnswers({})
        loadStatus()
      }
      if (msg.type === 'session_started') {
        sessionStorage.setItem(
          `session_${msg.payload.session_id}_questions`,
          JSON.stringify(msg.payload.questions)
        )
        navigate(`/room/${rid}/vote/${msg.payload.session_id}`)
      }
      if (msg.type === 'participant_joined' || msg.type === 'participant_left') {
        loadStatus()
      }
    })
    return disconnect
  }, [rid, participantId, sid, loadStatus, navigate])

  const handleSubmit = async () => {
    await api.submitAnswers(sid, participantId, answers)
    setSubmitted(true)
  }

  const handleReset = async () => {
    await api.resetSession(sid)
  }

  const handleBackToRoom = () => {
    navigate(`/room/${rid}`)
  }

  if (questions.length === 0) {
    return (
      <div className="container">
        <div className="card empty-state">
          <p>質問を読み込み中...</p>
          <p style={{ marginTop: 12, fontSize: '0.85rem', color: '#999' }}>
            表示されない場合は、戻って投票をやり直してください。
          </p>
          <button className="btn-secondary" onClick={handleBackToRoom} style={{ marginTop: 12 }}>
            ルームに戻る
          </button>
        </div>
      </div>
    )
  }

  return (
    <>
      <div className="header">
        <h1>投票</h1>
        <Link to={`/room/${rid}`} className="back-link">
          ルームに戻る
        </Link>
      </div>

      <div className="card">
        <h2>参加者</h2>
        <div className="participants-list">
          {statuses.map((s) => (
            <span
              key={s.participant_id}
              className={`participant-badge ${s.has_voted ? 'voted' : 'not-voted'}`}
            >
              {s.participant_name}: {s.has_voted ? '回答済' : '未回答'}
            </span>
          ))}
        </div>
      </div>

      {!revealed ? (
        <div className="card">
          <h2>あなたの回答 {submitted && '(送信済)'}</h2>
          {questions.map((q) => (
            <div key={q.id} className="form-group">
              <label>{q.text}</label>
              <input
                value={answers[String(q.id)] || ''}
                onChange={(e) =>
                  setAnswers((prev) => ({ ...prev, [String(q.id)]: e.target.value }))
                }
                disabled={submitted}
                placeholder="回答を入力..."
              />
            </div>
          ))}
          {!submitted ? (
            <button className="btn-primary" onClick={handleSubmit}>
              回答を送信
            </button>
          ) : (
            <p style={{ color: '#27ae60', fontWeight: 600 }}>
              他のメンバーの回答を待っています...
            </p>
          )}
        </div>
      ) : (
        <div className="card">
          <h2>結果</h2>
          <table className="results-table">
            <thead>
              <tr>
                <th>参加者</th>
                {questions.map((q) => (
                  <th key={q.id}>{q.text}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {results.map((r) => (
                <tr key={r.participant_id}>
                  <td style={{ fontWeight: 600 }}>{r.participant_name}</td>
                  {questions.map((q) => (
                    <td key={q.id}>{r.answers[String(q.id)] || '-'}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>

          <div className="btn-group" style={{ marginTop: 20 }}>
            <button className="btn-danger" onClick={handleReset}>
              再投票
            </button>
            <button className="btn-primary" onClick={handleBackToRoom}>
              次の見積もりへ
            </button>
          </div>
        </div>
      )}
    </>
  )
}
