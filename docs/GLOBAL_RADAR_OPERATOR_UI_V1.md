# Koschei Global Crypto Radar — Operator Surface v1

Date: 2026-09-23

## Goal

Turn the Global Crypto Radar architecture into an operator-visible surface without inventing production scale.

The page is available at:

`GET /fabric/radar`

The machine-readable snapshot remains:

`GET /fabric/radar/global`

## What the page shows

- the number of networks actually registered in the repository catalog;
- implementation-ready adapter/probe count;
- deployment configuration requirements;
- live-check state;
- network families and per-network runtime state;
- the versioned Radar Event -> Entity Graph -> Attack Path -> Security Evidence -> ARVIS pipeline.

## What the page deliberately does not show

- fabricated transaction totals;
- fabricated wallet or contract totals;
- fake bridge coverage numbers;
- inferred wallet/user physical locations;
- a claim that every registered network is being scanned 24/7.

The center radar visualization is decorative. It is explicitly not a geographic wallet map.

## Trust boundary

The UI consumes the same server-side Global Radar snapshot used by the JSON endpoint. It does not calculate security decisions in the browser.

A capability existing in code is still different from:

1. deployment configuration;
2. live availability;
3. observed evidence;
4. verified evidence;
5. an ARVIS deterministic verdict.

That separation remains visible in the operator surface.
