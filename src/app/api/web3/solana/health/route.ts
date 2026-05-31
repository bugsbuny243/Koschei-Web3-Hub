import { NextResponse } from "next/server";
export async function GET(){
 const rpc=process.env.SOLANA_RPC_URL?.trim();
 if(!rpc) return NextResponse.json({ok:false,error:"SOLANA_RPC_URL is not configured."},{status:503});
 try { const response=await fetch(rpc,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({jsonrpc:"2.0",id:1,method:"getVersion"}),signal:AbortSignal.timeout(8000),cache:"no-store"});
  if(!response.ok) throw new Error(`RPC request failed with status ${response.status}.`);
  const data=await response.json() as {result?:unknown;error?:{message?:string}};
  if(data.error) throw new Error(data.error.message || "RPC returned an error.");
  return NextResponse.json({ok:true,network:process.env.SOLANA_NETWORK?.trim()||"devnet",provider:process.env.WEB3_PROVIDER?.trim()||"custom",result:data.result});
 } catch(error){return NextResponse.json({ok:false,error:error instanceof Error?error.message:"Solana RPC request failed."},{status:502});}
}
