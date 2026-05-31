import { AuthForm } from "@/components/AuthForm";
import { SiteHeader } from "@/components/SiteHeader";
export default function SignupPage() { return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-lg px-5 py-16 lg:px-8"><p className="eyebrow">Builder onboarding</p><h1 className="mt-4 text-4xl font-black text-white">Get started</h1><p className="mt-4 text-sm leading-7 text-slate-400">Create a standard member account. Admin and owner access are never granted through public signup.</p><AuthForm mode="signup" /></section></main>; }
