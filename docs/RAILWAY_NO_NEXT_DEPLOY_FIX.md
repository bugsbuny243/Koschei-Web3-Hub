# Railway No-Next Deploy Fix

## What was fixed

- Railway build strategy was switched from the old `apps/web` Next.js command path to root-level Dockerfile deployment.
- Root `Dockerfile` now builds and serves `koschei/frontend` (React + TypeScript + Vite) as the production web output.
- `railway.toml` was updated to use the Dockerfile builder so Railway root directory can remain `/`.
- This disables the accidental deployment path that previously pointed to the old `apps/web` Next.js skeleton.

## Result

Railway deploy now serves the premium Koschei frontend from `koschei/frontend/dist` instead of the older minimal/incorrect page path.
