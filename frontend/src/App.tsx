import { useEffect } from 'react'
import { Routes, Route, useLocation } from 'react-router-dom'
import TopPage from './pages/TopPage'
import RoomPage from './pages/RoomPage'
import QuestionSetEditor from './pages/QuestionSetEditor'
import VotingPage from './pages/VotingPage'
import { api } from './api'

export default function App() {
  const location = useLocation()

  useEffect(() => {
    if (!location.pathname.startsWith('/room/')) return
    const onPageHide = () => {
      const pid = Number(sessionStorage.getItem('participant_id'))
      const match = location.pathname.match(/^\/room\/(\d+)/)
      const rid = match ? Number(match[1]) : 0
      if (pid && rid) api.leaveRoom(rid, pid)
    }
    window.addEventListener('pagehide', onPageHide)
    return () => window.removeEventListener('pagehide', onPageHide)
  }, [location.pathname])

  return (
    <div className="container">
      <Routes>
        <Route path="/" element={<TopPage />} />
        <Route path="/room/:roomId" element={<RoomPage />} />
        <Route path="/room/:roomId/question-set" element={<QuestionSetEditor />} />
        <Route path="/room/:roomId/question-set/:qsId" element={<QuestionSetEditor />} />
        <Route path="/room/:roomId/vote/:sessionId" element={<VotingPage />} />
      </Routes>
    </div>
  )
}
