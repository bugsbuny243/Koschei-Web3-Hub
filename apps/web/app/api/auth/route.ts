import { NextRequest, NextResponse } from "next/server";
import { createToken, ensureUsersTable, hashPassword, validatePassword } from "@/lib/auth";

export async function POST(req: NextRequest) {
  try {
    const { action, email, password } = await req.json();
    if (!email || !password) {
      return NextResponse.json({ error: "Email and password are required" }, { status: 400 });
    }

    const db = await ensureUsersTable();

    if (action === "register") {
      const passwordHash = hashPassword(password);
      const created = await db.query(
        `INSERT INTO users (email, password_hash, credits)
         VALUES ($1, $2, 100)
         ON CONFLICT (email) DO NOTHING
         RETURNING id, email, credits, role`,
        [email.toLowerCase(), passwordHash],
      );
      if (!created.rowCount) {
        return NextResponse.json({ error: "Email already exists" }, { status: 409 });
      }
      const token = createToken(created.rows[0]);
      return NextResponse.json({ token, user: created.rows[0] });
    }

    if (action === "login") {
      const found = await db.query("SELECT id, email, credits, role, password_hash FROM users WHERE email = $1", [email.toLowerCase()]);
      const user = found.rows[0];
      if (!user || !validatePassword(password, user.password_hash)) {
        return NextResponse.json({ error: "Invalid credentials" }, { status: 401 });
      }
      const token = createToken({ id: user.id, email: user.email, credits: user.credits, role: user.role });
      return NextResponse.json({ token, user: { id: user.id, email: user.email, credits: user.credits, role: user.role } });
    }

    return NextResponse.json({ error: "Unsupported action" }, { status: 400 });
  } catch (error) {
    console.error("auth error", error);
    return NextResponse.json({ error: "Authentication failed" }, { status: 500 });
  }
}
