# Database migrations

Apply the SQL files in `migrations/` in order to the server-side PostgreSQL database.

Set `DATABASE_URL` only in the server environment. The application uses the database connection string on the Go backend and does not send it to browser code.
