type WSHandler = (msg: { type: string; payload: any }) => void;

export function connectWS(
  roomId: number,
  participantId: number,
  onMessage: WSHandler
): () => void {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${protocol}//${window.location.host}/ws?room_id=${roomId}&participant_id=${participantId}`;

  let ws: WebSocket | null = null;
  let closed = false;

  function connect() {
    ws = new WebSocket(url);

    ws.onopen = () => {
      console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        onMessage(msg);
      } catch (e) {
        console.error('Failed to parse WS message:', e);
      }
    };

    ws.onclose = () => {
      if (!closed) {
        console.log('WebSocket disconnected, reconnecting in 2s...');
        setTimeout(connect, 2000);
      }
    };

    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
      ws?.close();
    };
  }

  connect();

  return () => {
    closed = true;
    ws?.close();
  };
}
