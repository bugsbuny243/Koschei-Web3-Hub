# Phase 3 Notes

Missing backend endpoints detected for requested frontend behavior:

- `POST /api/auth/register` is not currently exposed by Go API.
- `POST /api/auth/login` is not currently exposed by Go API.
- No streaming chat endpoint is currently exposed (for SSE/WebSocket streaming responses).

Frontend now surfaces these as API errors instead of faking success.
