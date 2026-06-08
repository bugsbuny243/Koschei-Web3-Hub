-- Normalize the member dashboard inventory so every visible module has one canonical card.
-- Legacy Pro/Beta aliases stay routable where implemented, but they are not active dashboard modules.
INSERT INTO koschei_modules (module_key, title, description, category, is_public, admin_only, status) VALUES
('wallet_score', 'Wallet Score', 'Core wallet reputation scoring based on public on-chain activity and trust signals.', 'Core Tools', false, false, 'active'),
('token_scanner', 'Token Scanner', 'Core token authority, holder, supply and rug-risk scanner.', 'Core Tools', false, false, 'active'),
('portfolio_tracker', 'Portfolio Tracker', 'Core portfolio balance tracker across public wallets.', 'Core Tools', false, false, 'active'),
('smart_money', 'Smart Money', 'Core smart-money style public signal review.', 'Core Tools', false, false, 'active'),
('watchlist', 'Watchlist', 'Single canonical wallet and contract monitoring module using public API polling.', 'Core Tools', false, false, 'active'),
('risk_scanner', 'Risk Scanner', 'Core risk scan endpoint for wallets, tokens, projects and public interaction targets.', 'Core Tools', false, false, 'active'),
('tx_decoder', 'TX Decoder', 'Single canonical transaction decoder card; Pro/Beta depth is exposed inside the tool page.', 'Core Tools', false, false, 'active'),
('metadata_studio', 'Metadata Studio', 'Builder metadata generator for projects, tokens, NFTs, grants and public goods.', 'Builder Tools', false, false, 'active'),
('builder_funding_assistant', 'Builder Funding Assistant', 'Builder-focused grant and funding application assistant.', 'Builder Tools', false, false, 'active'),
('developer_docs', 'Developer Docs', 'Developer documentation and implementation guides.', 'Builder Tools', true, false, 'active'),
('agent_api', 'Agent API', 'Scoped read-only API endpoints for agents and external tools.', 'Builder Tools', false, false, 'active'),
('api_docs', 'API Docs', 'Copyable public and authenticated API reference documentation.', 'Builder Tools', true, false, 'active'),
('project_radar', 'Project Radar', 'Early Solana/Web3 project discovery with public-data signals, risk hints and opportunity scoring.', 'Advanced Intelligence', false, false, 'active'),
('intelligence_graph', 'Intelligence Graph', 'Watchlist and event relationship intelligence graph.', 'Advanced Intelligence', false, false, 'active'),
('risk_engine_v2', 'Risk Engine v2', 'Pro/Beta combined risk intelligence across wallet, token, transaction and project context.', 'Advanced Intelligence', false, false, 'active'),
('cross_chain_risk', 'Cross-chain Risk Monitor', 'Preliminary bridge, route and cross-chain risk checklist.', 'Advanced Intelligence', false, false, 'active'),
('sybil_check', 'Sybil Check', 'Lightweight public-information anti-abuse and Sybil risk checklist.', 'Advanced Intelligence', false, false, 'active'),
('program_scanner', 'Program Scanner', 'Public Solana program metadata and upgrade-authority risk scanner.', 'Advanced Intelligence', false, false, 'active'),
('chain_health', 'Chain Health', 'Public-safe network and provider health dashboard.', 'Advanced Intelligence', false, false, 'active'),
('rug_radar', 'Rug Radar', 'Community-driven token launch and rug-risk radar.', 'Advanced Intelligence', true, false, 'active'),
('account', 'Account', 'Member profile, plan and output balance page.', 'Account', false, false, 'active'),
('pricing', 'Pricing', 'Credit packs and plan purchase page.', 'Account', true, false, 'active'),
('subscription', 'Subscription', 'Subscription and entitlement status surface.', 'Account', false, false, 'active')
ON CONFLICT (module_key) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    category = EXCLUDED.category,
    is_public = EXCLUDED.is_public,
    admin_only = EXCLUDED.admin_only,
    status = EXCLUDED.status,
    updated_at = now();

-- Archive duplicate or legacy aliases that previously caused repeated dashboard concepts.
UPDATE koschei_modules
SET status = 'archived',
    description = 'Archived dashboard alias; replaced by the canonical dashboard module inventory.',
    updated_at = now()
WHERE module_key IN ('grant_autopilot', 'public_sdk', 'sybil_layer', 'tx_decoder_pro', 'x402_pay_per_tool');

-- Keep tool pricing labels aligned with the canonical visible cards.
INSERT INTO tool_usage_prices(tool_key, display_name, price_usd, payment_mode) VALUES
('risk_scanner', 'Risk Scanner', 0, 'disabled'),
('tx_decoder', 'TX Decoder', 0, 'disabled'),
('metadata_studio', 'Metadata Studio', 0, 'disabled'),
('watchlist', 'Watchlist', 0, 'disabled'),
('builder_funding_assistant', 'Builder Funding Assistant', 0, 'disabled'),
('chain_health', 'Chain Health', 0, 'disabled'),
('rug_radar', 'Rug Radar', 0, 'disabled'),
('program_scanner', 'Program Scanner', 0, 'disabled')
ON CONFLICT (tool_key) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    payment_mode = EXCLUDED.payment_mode,
    updated_at = now();
