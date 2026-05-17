import NextAuth from "next-auth";
import Credentials from "next-auth/providers/credentials";
import { PrismaAdapter } from "@auth/prisma-adapter";
import { prisma } from "@/lib/prisma";

export const { handlers, auth, signIn, signOut } = NextAuth({
  adapter: PrismaAdapter(prisma),
  secret: process.env.AUTH_SECRET,
  trustHost: true,
  providers: [
    Credentials({
      name: "Demo Login",
      credentials: { email: {}, name: {} },
      async authorize(credentials) {
        if (!credentials?.email || typeof credentials.email !== "string") return null;
        const email = credentials.email.toLowerCase();
        let user = await prisma.user.findUnique({ where: { email } });
        if (!user) {
          user = await prisma.user.create({ data: { email, name: String(credentials?.name || "Koschei User") } });
        }
        return user;
      }
    })
  ],
  pages: { signIn: "/" }
});
