import { AuthForm } from "@/components/AuthForm";
import { SiteHeader } from "@/components/SiteHeader";
export default function LoginPage() { return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-lg px-5 py-16 lg:px-8"><p className="eyebrow">Member access</p><h1 className="mt-4 text-4xl font-black text-white">Sign in</h1><p className="mt-4 text-sm leading-7 text-slate-400">Access your Koschei package rights and builder shortcuts.</p><AuthForm mode="login" /></section></main>; }
