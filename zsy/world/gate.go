package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// =========================================================================
// ★★ The validity gate (docs/23 §10 item 10 — decided: option A)
//
// Every mutation the engine performs is checked against the **authoritative**
// validator before it is allowed to reach the database. A document that does not
// pass never gets stored, and the caller is told what would have been wrong.
//
// # Why this exists
//
// Measured, not theorised: `retype` changes an element's type in the registry but
// leaves its `metadata` alone, so the re-typed element keeps keys its new type's
// `field_template` does not declare and misses the fields the new type requires.
// `validate.mjs` reports E14 ×3 + E15 ×2 for exactly that (docs/23 §12.3). The
// engine's own structural check says "clean", so without this gate the server
// would happily store a world that the authority rejects — and bad data would
// flow into storyboards.
//
// # Why Go can own this without breaking the one-rule rule
//
// The gate decides nothing about what a legal document is. It *asks the
// authority* and reads its verdict — the same separation as `world.validate`.
// The two rejected designs were rejected for exactly this reason:
//
//	B. reconcile `metadata` here  → that IS writing a rule, and "what should the
//	                                new required field contain" has no correct
//	                                answer (规范 19.4.1 puts it with human editing)
//	C. warn but store anyway      → contradicts docs/23 §5.2 ("服务端才是权威")
//
// ⚠ What it deliberately does NOT do: upgrade warnings to errors. Warnings (W1–W9)
// are quality signals; refusing on them would block legitimate edits. Only the
// error layer blocks, which is what `verdict.OK` means.
// =========================================================================

// authorityVerdict is the part of the validator's answer the gate needs.
//
// ⚠ It carries no judgement of its own — just enough to decide "block or pass"
// and to explain why in the caller's language.
type authorityVerdict struct {
	// OK is the authority's own verdict: shape layer passed AND no 18.3 errors.
	OK bool
	// ShapeOK is the JSON-Schema layer.
	ShapeOK bool
	// ShapeErrors are Ajv's messages, verbatim.
	ShapeErrors []string
	// Errors are the 18.3 error-level issues, verbatim.
	Errors []worldIssue
	// Warnings are reported to the client but never block.
	Warnings []worldIssue
}

// Summary renders the verdict as one clause a client can act on.
//
// ★ It quotes the authority rather than paraphrasing it: the client shows this
// next to "这次改动会让世界不合法". A paraphrase here would be a second description
// of the rule, and it would drift.
func (v authorityVerdict) Summary() string {
	parts := make([]string, 0, 4)
	for _, e := range v.Errors {
		parts = append(parts, fmt.Sprintf("%s %s", e.Code, e.Msg))
		if len(parts) >= 3 {
			break
		}
	}
	if len(v.Errors) > len(parts) {
		parts = append(parts, fmt.Sprintf("…共 %d 条", len(v.Errors)))
	}
	for _, e := range v.ShapeErrors {
		parts = append(parts, e)
		if len(parts) >= 3 {
			break
		}
	}
	if len(parts) == 0 {
		// No rule name to quote — say what is knowable, not a fake specific.
		return "权威校验器未通过这份文档（未给出具体条目）"
	}
	return strings.Join(parts, "；")
}

// gateOnAuthority runs the authoritative validator and reports its verdict.
//
// An error return means the *validator itself* was unreachable — a deployment
// problem (E_UPSTREAM), which is a different thing from "the document is bad"
// (E_INPUT raised by the caller). Keeping those apart is the whole reason the
// gate returns both a verdict and an error.
func gateOnAuthority(ctx context.Context, doc json.RawMessage) (authorityVerdict, error) {
	report, err := runValidatorSidecar(ctx, doc)
	if err != nil {
		return authorityVerdict{}, err
	}

	var parsed validatorReport
	if err := common.Unmarshal(report, &parsed); err != nil {
		return authorityVerdict{}, fmt.Errorf("world: gate: validator payload is not parseable: %w", err)
	}
	if len(parsed.Report) == 0 {
		return authorityVerdict{}, fmt.Errorf("world: gate: validator returned an empty report")
	}
	entry := parsed.Report[0]

	return authorityVerdict{
		OK:          entry.ShapeOK && len(entry.Errors) == 0,
		ShapeOK:     entry.ShapeOK,
		ShapeErrors: nonNilStrings(entry.ShapeErrors),
		Errors:      nonNilIssues(entry.Errors),
		Warnings:    nonNilIssues(entry.Warnings),
	}, nil
}
