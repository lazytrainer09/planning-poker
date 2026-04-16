import { Routes, Route } from 'react-router-dom'
import TopPage from './pages/TopPage'
import RoomPage from './pages/RoomPage'
import QuestionSetEditor from './pages/QuestionSetEditor'
import VotingPage from './pages/VotingPage'

export default function App() {
  return (
    <div className="container">
      <Routes>
        <Route path="/" element={<TopPage />} />
        <Route path="/room/:roomId" element={<RoomPage />} />
        <Route path="/room/:roomId/question-set/:qsId?" element={<QuestionSetEditor />} />
        <Route path="/room/:roomId/vote/:sessionId" element={<VotingPage />} />
      </Routes>
    </div>
  )
}
