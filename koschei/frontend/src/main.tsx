import React, { useState } from 'react'
import { createRoot } from 'react-dom/client'

function App() {
  const [message, setMessage] = useState('')
  const [response, setResponse] = useState('')

  const send = async () => {
    const res = await fetch('http://localhost:8080/api/chat', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ message }) })
    const data = await res.json(); setResponse(`${data.model}: ${data.response || data.error}`)
  }

  return <div style={{fontFamily:'sans-serif',padding:24}}><h1>KOSCHEI AI Super App</h1><textarea value={message} onChange={e=>setMessage(e.target.value)} /><br/><button onClick={send}>Send</button><pre>{response}</pre></div>
}

createRoot(document.getElementById('root')!).render(<App />)
