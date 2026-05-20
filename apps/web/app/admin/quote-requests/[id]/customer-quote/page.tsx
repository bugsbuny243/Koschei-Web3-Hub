export default async function CustomerQuoteCalculatorPage({params,searchParams}:{params:Promise<{id:string}>;searchParams:Promise<{password?:string}>}){
const {id}=await params; const {password}=await searchParams;
if(!process.env.ADMIN_PASSWORD||password!==process.env.ADMIN_PASSWORD)return <div className="page-stack"><h1>Admin Access Required</h1></div>;
return <div className="page-stack"><h1>Customer Quote Calculator</h1><form className="card" action={`/api/admin/quote-requests/${id}/customer-quote?password=${password}`} method="post">
<input name="supplier_ddp_price_usd" placeholder="supplier_ddp_price_usd" className="input" required/>
<input name="escrow_fee_internal" placeholder="escrow_fee_internal" className="input" required/>
<input name="bank_transfer_fee_internal" placeholder="bank_transfer_fee_internal" className="input" required/>
<input name="operation_cost_internal" placeholder="operation_cost_internal" className="input" required/>
<select name="commission_type" className="input"><option value="fixed">fixed</option><option value="percent">percent</option></select>
<input name="commission_fixed_usd" placeholder="commission_fixed_usd" className="input" />
<input name="commission_percent" placeholder="commission_percent" className="input" />
<input name="payment_terms_public" placeholder="payment_terms_public" className="input" />
<input name="delivery_terms_public" placeholder="delivery_terms_public" className="input" />
<input name="valid_until" type="date" className="input" />
<textarea name="quote_notes" placeholder="quote_notes" className="input" />
<button className="btn btn-primary" type="submit">Save customer quote</button></form></div>
}
