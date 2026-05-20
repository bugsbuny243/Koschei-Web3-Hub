function toNumber(value: string | undefined) {
  const parsed = Number(value ?? 0);
  return Number.isFinite(parsed) ? parsed : 0;
}

function usd(amount: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(amount);
}

export default async function CustomerQuoteCalculatorPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ password?: string; supplier_landed_cost?: string; escrow_fee_internal?: string; bank_transfer_fee_internal?: string; operation_cost_internal?: string; markup_mode?: string; markup_amount?: string; markup_percent?: string }>;
}) {
  const { id } = await params;
  const input = await searchParams;

  if (!process.env.ADMIN_PASSWORD || input.password !== process.env.ADMIN_PASSWORD) {
    return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  }

  const supplierLandedCost = toNumber(input.supplier_landed_cost);
  const escrowFeeInternal = toNumber(input.escrow_fee_internal);
  const bankTransferFeeInternal = toNumber(input.bank_transfer_fee_internal);
  const operationCostInternal = toNumber(input.operation_cost_internal);
  const internalTotalCost = supplierLandedCost + escrowFeeInternal + bankTransferFeeInternal + operationCostInternal;

  const markupMode = input.markup_mode === "percent" ? "percent" : "fixed";
  const markupAmount = toNumber(input.markup_amount);
  const markupPercent = toNumber(input.markup_percent);

  const finalCustomerQuote = markupMode === "percent"
    ? internalTotalCost * (1 + markupPercent / 100)
    : internalTotalCost + markupAmount;

  const grossProfit = finalCustomerQuote - internalTotalCost;
  const grossMargin = finalCustomerQuote > 0 ? (grossProfit / finalCustomerQuote) * 100 : 0;

  return (
    <div className="page-stack">
      <h1>Customer Quote Calculator (Admin)</h1>
      <p><strong>Quote request ID:</strong> {id}</p>
      <p className="rounded-xl border border-amber-300 bg-amber-50 p-3 text-sm">
        Escrow.com fees are paid by TradePi Globall. They are included as internal cost and must not be charged separately to customer or supplier.
      </p>

      <ul>
        <li><strong>Supplier landed cost:</strong> {usd(supplierLandedCost)}</li>
        <li><strong>Escrow.com fee paid by TradePi Globall:</strong> {usd(escrowFeeInternal)}</li>
        <li><strong>Bank/wire transfer fee paid by TradePi Globall:</strong> {usd(bankTransferFeeInternal)}</li>
        <li><strong>Other operation cost:</strong> {usd(operationCostInternal)}</li>
        <li><strong>Internal total cost:</strong> {usd(internalTotalCost)}</li>
        <li><strong>Markup ({markupMode === "percent" ? `${markupPercent}%` : "fixed"}):</strong> {markupMode === "percent" ? `${markupPercent}%` : usd(markupAmount)}</li>
        <li><strong>Final customer quote:</strong> {usd(finalCustomerQuote)}</li>
        <li><strong>Gross profit:</strong> {usd(grossProfit)}</li>
        <li><strong>Gross margin:</strong> {grossMargin.toFixed(2)}%</li>
      </ul>

      <p className="text-sm text-slate-700">
        Supplier payment tracking remains separate: T/T 30% advance payment and T/T 70% balance before delivery.
        Escrow fee must not reduce supplier payment amount.
      </p>
    </div>
  );
}
