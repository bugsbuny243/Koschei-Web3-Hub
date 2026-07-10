package services

// securityRadarPublicFeedExcludedMintsSQL contains established Solana
// infrastructure and blue-chip mints that are still persisted for internal
// intelligence, but must not be presented as public Security Radar findings.
const securityRadarPublicFeedExcludedMintsSQL = `ARRAY[
	'So11111111111111111111111111111111111111112',
	'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v',
	'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB',
	'JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN',
	'4k3Dyjzvzp8eMZWUXbBCjEvwSkkk59S5iCNLY3QrkX6R',
	'DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263',
	'7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs',
	'mSoLzYCxHdYgdzU16g5QSh3i5K3z3KZK7ytfqcJm7So',
	'7dHbWXmci3dT8UFYWYZweBLXgycu7Y3iL6trKn1Y7ARj',
	'J1toso1uCk3RLmjorhTtrVwY9HJ7X8V9yYac6Y7kGCPn',
	'bSo13r4TkiE4KumL71LsHTPpL2euBYLFx6h9HP3piy1',
	'HZ1JovNiVvGrGNiiYvEozEVgZ58xaU3RKwX8eACQBCt3',
	'orcaEKTdK7LKz57vaAYr9QeNsVEPfiu6QeMU1kektZE',
	'jtojtomepa8beP8AuQc6eXt5FriJwfFMwQx2v2f9mCL'
]::text[]`
