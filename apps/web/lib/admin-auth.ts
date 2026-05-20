export function isAdminAuthed(password: string | null) {
  return !!process.env.ADMIN_PASSWORD && password === process.env.ADMIN_PASSWORD;
}
