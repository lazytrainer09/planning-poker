import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, Room } from '../api'

export default function TopPage() {
  const navigate = useNavigate()
  const [rooms, setRooms] = useState<Room[]>([])

  // Create room
  const [newName, setNewName] = useState('')
  const [newPass, setNewPass] = useState('')

  // Login
  const [loginMode, setLoginMode] = useState<'select' | 'name'>('select')
  const [loginRoomId, setLoginRoomId] = useState<number | ''>('')
  const [loginRoomName, setLoginRoomName] = useState('')
  const [loginPass, setLoginPass] = useState('')
  const [loginName, setLoginName] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.listRooms().then(setRooms).catch(() => {})
  }, [])

  const handleCreate = async () => {
    setError('')
    try {
      const room = await api.createRoom(newName, newPass)
      setRooms((prev) => [{ id: room.id, name: room.name }, ...prev])
      setNewName('')
      setNewPass('')
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleLogin = async () => {
    setError('')
    if (loginMode === 'select' && !loginRoomId) return
    if (loginMode === 'name' && !loginRoomName.trim()) return
    try {
      const res = await api.login(
        loginMode === 'select' ? (loginRoomId as number) : 0,
        loginMode === 'name' ? loginRoomName.trim() : '',
        loginPass,
        loginName,
      )
      sessionStorage.setItem('participant_id', String(res.participant_id))
      sessionStorage.setItem('participant_name', loginName)
      sessionStorage.setItem('room_name', res.room_name)
      navigate(`/room/${res.room_id}`)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const selectStyle = {
    width: '100%',
    padding: '10px 14px',
    border: '2px solid #e0e0e0',
    borderRadius: '8px',
    fontSize: '1rem',
  }

  return (
    <>
      <h1>プランニングポーカー</h1>
      {error && (
        <div className="card" style={{ background: '#fdebd0', color: '#e74c3c' }}>
          {error}
        </div>
      )}

      <div className="two-col">
        <div className="card">
          <h2>ルーム作成</h2>
          <div className="form-group">
            <label>ルーム名</label>
            <input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="例: スプリントプランニング"
            />
          </div>
          <div className="form-group">
            <label>合言葉</label>
            <input
              type="password"
              value={newPass}
              onChange={(e) => setNewPass(e.target.value)}
              placeholder="合言葉を入力"
            />
          </div>
          <button className="btn-primary" onClick={handleCreate}>
            作成
          </button>
        </div>

        <div className="card">
          <h2>ルームに参加</h2>
          <div className="form-group">
            <label>ルーム</label>
            <div className="btn-group" style={{ marginBottom: 8 }}>
              <button
                className={loginMode === 'select' ? 'btn-primary btn-sm' : 'btn-secondary btn-sm'}
                onClick={() => setLoginMode('select')}
              >
                一覧から選択
              </button>
              <button
                className={loginMode === 'name' ? 'btn-primary btn-sm' : 'btn-secondary btn-sm'}
                onClick={() => setLoginMode('name')}
              >
                ルーム名で入力
              </button>
            </div>
            {loginMode === 'select' ? (
              <select
                value={loginRoomId}
                onChange={(e) => setLoginRoomId(e.target.value ? Number(e.target.value) : '')}
                style={selectStyle}
              >
                <option value="">ルームを選択...</option>
                {rooms.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            ) : (
              <input
                value={loginRoomName}
                onChange={(e) => setLoginRoomName(e.target.value)}
                placeholder="ルーム名を入力"
              />
            )}
          </div>
          <div className="form-group">
            <label>合言葉</label>
            <input
              type="password"
              value={loginPass}
              onChange={(e) => setLoginPass(e.target.value)}
              placeholder="合言葉を入力"
            />
          </div>
          <div className="form-group">
            <label>あなたの名前</label>
            <input
              value={loginName}
              onChange={(e) => setLoginName(e.target.value)}
              placeholder="例: 太郎"
            />
          </div>
          <button className="btn-success" onClick={handleLogin}>
            参加
          </button>
        </div>
      </div>
    </>
  )
}
