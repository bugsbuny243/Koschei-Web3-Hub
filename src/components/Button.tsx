import Link from "next/link";
import type { ButtonHTMLAttributes, ReactNode } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  href?: string;
  variant?: "primary" | "secondary" | "ghost";
  className?: string;
};

const variants = {
  primary: "bg-cyan-500 text-slate-950 hover:bg-cyan-400 shadow-sm shadow-cyan-950/10",
  secondary: "bg-white text-slate-900 ring-1 ring-slate-200 hover:bg-slate-50",
  ghost: "bg-transparent text-slate-600 hover:bg-slate-100",
};

export function Button({ children, href, variant = "primary", className = "", ...props }: Props) {
  const classes = `inline-flex min-h-11 cursor-pointer items-center justify-center rounded-xl px-5 py-3 text-center text-sm font-bold ${variants[variant]} ${className}`;
  if (href) return <Link href={href} className={classes}>{children}</Link>;
  return <button className={classes} {...props}>{children}</button>;
}
