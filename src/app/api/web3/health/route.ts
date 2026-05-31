import { NextResponse } from "next/server"; import { checkChainHealth, isSupportedChain } from "@/lib/web3/alchemy";
export async function GET(request:Request){const chain=new URL(request.url).searchParams.get("chain")||"";if(!isSupportedChain(chain))return NextResponse.json({ok:false,error:"Unsupported chain."},{status:400});return NextResponse.json(await checkChainHealth(chain));}
