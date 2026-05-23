# Phase 3 Notes

- Auth endpoints are present in Go API (`/api/auth/register`, `/api/auth/login`, `/api/me`) and token-auth protected private endpoints are wired.
- Streaming response endpoint is not currently present in Go API routes.
- Chat endpoint (`/api/chat`) is not present; dashboard currently uses runtime flow (`/api/runtime/projects`, `/api/runtime/tasks`, `/api/runtime/logs/:projectId`).
- Expo frontend uses `process.env.EXPO_PUBLIC_API_URL` with same-origin fallback when unset.
