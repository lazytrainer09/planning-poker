import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '../api'
import { connectWS } from '../ws'

interface QuestionInput {
  text: string
  sort_order: number
}

export default function QuestionSetEditor() {
  const { roomId, qsId } = useParams<{ roomId: string; qsId?: string }>()
  const navigate = useNavigate()
  const rid = Number(roomId)
  const editId = qsId ? Number(qsId) : null

  const participantId = Number(sessionStorage.getItem('participant_id'))

  // Keep WS alive so participant is not deleted during editing
  useEffect(() => {
    const disconnect = connectWS(rid, participantId, () => {})
    return disconnect
  }, [rid, participantId])

  const [name, setName] = useState('')
  const [questions, setQuestions] = useState<QuestionInput[]>([
    { text: '', sort_order: 0 },
  ])

  useEffect(() => {
    if (!editId) return
    api.listQuestionSets(rid).then((sets) => {
      const qs = sets.find((s) => s.id === editId)
      if (qs) {
        setName(qs.name)
        setQuestions(
          qs.questions.length > 0
            ? qs.questions.map((q) => ({ text: q.text, sort_order: q.sort_order }))
            : [{ text: '', sort_order: 0 }]
        )
      }
    })
  }, [rid, editId])

  const addQuestion = () => {
    setQuestions((prev) => [...prev, { text: '', sort_order: prev.length }])
  }

  const removeQuestion = (idx: number) => {
    setQuestions((prev) => prev.filter((_, i) => i !== idx).map((q, i) => ({ ...q, sort_order: i })))
  }

  const updateQuestion = (idx: number, text: string) => {
    setQuestions((prev) =>
      prev.map((q, i) => (i === idx ? { ...q, text } : q))
    )
  }

  const moveQuestion = (idx: number, dir: -1 | 1) => {
    const newIdx = idx + dir
    if (newIdx < 0 || newIdx >= questions.length) return
    const copy = [...questions]
    ;[copy[idx], copy[newIdx]] = [copy[newIdx], copy[idx]]
    setQuestions(copy.map((q, i) => ({ ...q, sort_order: i })))
  }

  const handleSave = async () => {
    const validQuestions = questions.filter((q) => q.text.trim())
    if (!name.trim() || validQuestions.length === 0) return

    if (editId) {
      await api.updateQuestionSet(editId, name, validQuestions)
    } else {
      await api.createQuestionSet(rid, name, validQuestions)
    }
    navigate(`/room/${rid}`)
  }

  return (
    <>
      <div className="header">
        <h1>質問セット{editId ? '編集' : '作成'}</h1>
        <Link to={`/room/${rid}`} className="back-link">
          戻る
        </Link>
      </div>

      <div className="card">
        <div className="form-group">
          <label>セット名</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="例: 標準タスク"
          />
        </div>

        <h2>質問</h2>
        {questions.map((q, idx) => (
          <div key={idx} className="question-item">
            <span style={{ color: '#999', minWidth: 24 }}>{idx + 1}.</span>
            <input
              value={q.text}
              onChange={(e) => updateQuestion(idx, e.target.value)}
              placeholder="質問内容"
            />
            <button
              className="btn-secondary btn-sm"
              onClick={() => moveQuestion(idx, -1)}
              disabled={idx === 0}
            >
              上
            </button>
            <button
              className="btn-secondary btn-sm"
              onClick={() => moveQuestion(idx, 1)}
              disabled={idx === questions.length - 1}
            >
              下
            </button>
            <button
              className="btn-danger btn-sm"
              onClick={() => removeQuestion(idx)}
              disabled={questions.length <= 1}
            >
              削除
            </button>
          </div>
        ))}

        <div style={{ marginTop: 16 }} className="btn-group">
          <button className="btn-secondary" onClick={addQuestion}>
            + 質問を追加
          </button>
          <button className="btn-primary" onClick={handleSave}>
            保存
          </button>
        </div>
      </div>
    </>
  )
}
